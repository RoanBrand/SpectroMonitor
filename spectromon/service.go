package spectromon

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/RoanBrand/SpectroMonitor/config"
	"github.com/RoanBrand/SpectroMonitor/displayboard"
	"github.com/RoanBrand/SpectroMonitor/http"
	"github.com/RoanBrand/SpectroMonitor/lights"
	"github.com/RoanBrand/SpectroMonitor/log"
	"github.com/kardianos/service"
)

type app struct {
	conf *config.Config

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

	if err := displayboard.Start(a.conf.SerialPortName, a.conf.SerialBaudRate); err != nil {
		log.Fatal("could not open serial connection:", err)
	}

	a.furnaceLastResult = make(map[string]time.Duration)
	url := a.makeURL()
	go a.runGetSetTimeJob()
	go a.handleDisplayBoards()

	a.doTask(url)
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
	return nil
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

	for {
		select {
		case <-t.C:
			a.lock.Lock()
			for _, f := range a.conf.Furnaces {
				d, ok := a.furnaceLastResult[f.Name]
				if !ok {
					continue
				}

				if d > maxAge {
					d = maxAge
				}

				if err := displayboard.Write(f.DisplayBoardAddress, []byte(formatDuration(d, colon))); err != nil {
					log.Println("error writing to displayboard:", err)
				}
			}
			a.lock.Unlock()
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

	a.lock.Lock()
	defer a.lock.Unlock()

	now := time.Now()
	for i := range a.conf.Furnaces {
		f := &a.conf.Furnaces[i]
		for j := range res {
			resF := &res[j]

			if resF.Furnace != f.Name {
				continue
			}

			a.furnaceLastResult[f.Name] = now.Sub(resF.TimeStamp)
			if a.furnaceLastResult[f.Name] > maxAge {
				// red
				lights.SetLight(f.LightCardAddress, f.RedLightAddress)
				lights.ClearLight(f.LightCardAddress, f.GreenLightAddress)
			} else {
				// green
				lights.SetLight(f.LightCardAddress, f.GreenLightAddress)
				lights.ClearLight(f.LightCardAddress, f.RedLightAddress)
			}

			break
		}
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
