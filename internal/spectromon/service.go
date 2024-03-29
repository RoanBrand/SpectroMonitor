package spectromon

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/RoanBrand/SpectroMonitor/internal/config"
	"github.com/RoanBrand/SpectroMonitor/internal/deltaplc"
	"github.com/RoanBrand/SpectroMonitor/internal/http"
	"github.com/RoanBrand/SpectroMonitor/internal/log"

	"github.com/kardianos/service"
)

type app struct {
	conf       *config.Config
	deltaPLCIO *deltaplc.Modbus

	ctx        context.Context
	cancelFunc context.CancelFunc

	furnaceLastResult map[string]time.Duration
	lock              sync.Mutex
}

func New(c *config.Config) *app {
	return &app{conf: c}
}

func (a *app) Start(s service.Service) error {
	a.ctx, a.cancelFunc = context.WithCancel(context.Background())
	go a.startup()
	return nil
}

func (a *app) startup() {
	log.Setup(a.conf.LogFilePath, !service.Interactive())

	d, err := deltaplc.New(a.conf.ModbusURL)
	if err != nil {
		log.Fatal(err)
	}

	a.deltaPLCIO = d
	a.furnaceLastResult = make(map[string]time.Duration)
	url := a.makeURL()

	a.doTask(url)

	go a.runGetSetTimeJob()
	go a.handleDisplayBoards()

	interval := time.Duration(a.conf.RequestIntervalSeconds) * time.Second
	t := time.NewTimer(interval)

	for {
		select {
		case <-t.C:
			a.doTask(url)
			t.Reset(interval)
		case <-a.ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return
		}
	}
}

func (a *app) Stop(s service.Service) error {
	a.cancelFunc()
	return a.deltaPLCIO.Close()
}

func (a *app) runGetSetTimeJob() {
	if a.conf.TimeUpdateIntervalSeconds == 0 {
		return
	}

	interval := time.Duration(a.conf.TimeUpdateIntervalSeconds) * time.Second
	t := time.NewTimer(interval)

	for {
		select {
		case <-t.C:
			newTime, err := http.GetTime(a.conf.TimeUpdateUrl)
			if err != nil {
				log.Println("unable to get time from network:", err)
			} else {
				if err = setSystemDate(newTime); err != nil {
					log.Println("unable to update system time:", err)
				}
			}

			t.Reset(interval)
		case <-a.ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return
		}
	}
}

func (a *app) handleDisplayBoards() {
	interval := time.Duration(a.conf.DisplayBoardUpdateRateSeconds) * time.Second
	if interval == 0 {
		interval = 1
	}

	t := time.NewTimer(interval)
	maxAge := time.Duration(a.conf.FurnaceResultOldTimeMinutes) * time.Minute
	colon := true

	displayData := make([]byte, len(a.conf.Furnaces)*16)
	nonces := make([]uint8, len(a.conf.Furnaces))

	for {
		select {
		case <-t.C:
			a.lock.Lock()
			for i := range a.conf.Furnaces {
				f := &a.conf.Furnaces[i]

				d, ok := a.furnaceLastResult[f.Name]
				if !ok {
					continue
				}

				if d > maxAge {
					d = maxAge
				}

				msg := []byte(formatDuration(d, colon))
				addrOffSet := uint16(i * 16)
				makeDisplayStringRaw(
					f.DisplayBoardAddress,
					nonces[i],
					displayData[addrOffSet:addrOffSet],
					msg)

				nonces[i]++
			}
			a.lock.Unlock()

			err := a.deltaPLCIO.WriteBytes(a.conf.ModbusAddrDisplays, displayData)
			if err != nil {
				log.Println("failed to write display output data on delta PLC IO over Modbus:", err)
			}

			colon = !colon
			t.Reset(interval)

		case <-a.ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return
		}
	}
}

// get latest test samples for furnaces and update lights
func (a *app) doTask(url string) {
	res, err := http.GetResult(url, a.conf)
	if err != nil {
		log.Println(err)
		return
	}

	maxAge := time.Duration(a.conf.FurnaceResultOldTimeMinutes) * time.Minute

	coils := make([]bool, len(a.conf.Furnaces)*2)

	a.lock.Lock()

	now := time.Now()
	for i := range a.conf.Furnaces {
		f := &a.conf.Furnaces[i]
		addrOffSet := uint16(i * 2)

		for j := range res {
			resF := &res[j]

			if resF.Furnace != f.Name {
				continue
			}

			a.furnaceLastResult[f.Name] = now.Sub(resF.TimeStamp)
			if a.furnaceLastResult[f.Name] > maxAge {
				// red
				coils[addrOffSet] = true
				coils[addrOffSet+1] = false
			} else {
				// green
				coils[addrOffSet] = false
				coils[addrOffSet+1] = true
			}

			break
		}
	}
	a.lock.Unlock()

	if err = a.deltaPLCIO.WriteCoils(a.conf.ModbusAddrLights, coils); err != nil {
		log.Println("failed to set output coils for light on delta PLC IO over Modbus:", err)
	}
}

func (a *app) makeURL() string {
	url := strings.TrimSuffix(a.conf.ResultUrl, "/")
	url += "?"
	for i := range a.conf.Furnaces {
		url += "f=" + a.conf.Furnaces[i].Name + "&"
	}

	if a.conf.TransferSamplesOnly {
		url += "t=true"
	}

	return url
}

// whole msg must fit in dst's cap
func makeDisplayStringRaw(displayAddress, nonce uint8, dst, msg []byte) {
	//b := make([]byte, 0, 16)
	dst = append(dst, 0x0, 0x53, displayAddress, 0x3)

	dst = append(dst, msg...)
	dst = append(dst, 0x4)

	var newXor byte
	for _, dataByte := range dst {
		newXor ^= dataByte
	}
	dst = append(dst, newXor)

	// nonce byte is last byte written to plc that changes with each new message
	// so that plc can know when to read a new value
	dst = append(dst, nonce)

	//return b
}

func formatDuration(d time.Duration, withColon bool) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute

	if withColon {
		return fmt.Sprintf("%02d:%02d", h, m)
	} else {
		return fmt.Sprintf("%02d %02d", h, m)
	}
}

func setSystemDate(newTime time.Time) error {
	_, err := exec.LookPath("date")
	if err != nil {
		log.Printf("Date binary not found, cannot set system date: %s\n", err.Error())
		return err
	}

	dateString := newTime.Format("2 Jan 2006 15:04:05")
	log.Printf("Setting system date to: %s\n", dateString)
	args := []string{"--set", dateString}
	return exec.Command("date", args...).Run()
}
