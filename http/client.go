package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/RoanBrand/SpectroMonitor/config"
	"github.com/RoanBrand/SpectroMonitor/displayboard"
	"github.com/RoanBrand/SpectroMonitor/lights"
	"github.com/RoanBrand/SpectroMonitor/log"
)

var furnaceLastResult map[string]time.Duration
var lock sync.Mutex

func Start(conf *config.Config) {
	furnaceLastResult = make(map[string]time.Duration)

	go func() {
		if err := getTime(conf.TimeUpdateUrl); err != nil {
			log.Println("unable to update time:", err)
		}
		time.Sleep(time.Duration(conf.TimeUpdateIntervalSeconds) * time.Second)
	}()

	url := strings.TrimSuffix(conf.ResultUrl, "/")
	url += "?"
	for i := range conf.Furnaces {
		url += "f=" + conf.Furnaces[i].Name + "&"
	}

	if conf.TransferSamplesOnly {
		url += "t=true"
	}

	go handleDisplayBoards(conf)

	doTask(url, conf)
	for {
		time.Sleep(time.Duration(conf.RequestIntervalSeconds) * time.Second)
		doTask(url, conf)
	}
}

func doTask(url string, conf *config.Config) {
	res, err := getResult(url, conf)
	if err != nil {
		log.Println(err)
	}

	t := time.Now()
	lock.Lock()
	defer lock.Unlock()

	for _, f := range conf.Furnaces {
		for _, resF := range res {
			if resF.Furnace != f.Name {
				continue
			}

			furnaceLastResult[f.Name] = t.Sub(resF.TimeStamp)
			if int(furnaceLastResult[f.Name].Seconds()) > (conf.FurnaceResultOldTimeMinutes * 60) {
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

func handleDisplayBoards(conf *config.Config) {
	colon := true
	for {
		time.Sleep(time.Duration(conf.DisplayBoardUpdateRateSeconds) * time.Second)
		lock.Lock()
		for _, f := range conf.Furnaces {
			d, ok := furnaceLastResult[f.Name]
			if !ok {
				continue
			}

			td := time.Duration(conf.FurnaceResultOldTimeMinutes)*time.Minute
			if d > td {
				d = td
			}

			if err := displayboard.Write(f.DisplayBoardAddress, []byte(formatDuration(d, colon))); err != nil {
				log.Println("error writing to displayboard:", err)
			}
		}
		lock.Unlock()
		colon = !colon
	}

}

type resultResponse struct {
	SampleName string    `json:"sample_name"`
	Furnace    string    `json:"furnace"`
	TimeStamp  time.Time `json:"time_stamp"`
}

func getResult(url string, conf *config.Config) ([]resultResponse, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("request error: " + resp.Status)
	}

	defer resp.Body.Close()
	res := make([]resultResponse, 0, len(conf.Furnaces))

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&res); err != nil {
		return nil, err
	}

	return res, nil
}

func getTime(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("gettime request error: " + resp.Status)
	}

	defer resp.Body.Close()
	res := struct {
		T time.Time `json:"t"`
	}{}

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&res); err != nil {
		return err
	}

	return setSystemDate(res.T)
}

func setSystemDate(newTime time.Time) error {
	_, lookErr := exec.LookPath("date")
	if lookErr != nil {
		log.Printf("Date binary not found, cannot set system date: %s\n", lookErr.Error())
		return lookErr
	} else {
		dateString := newTime.Format("2 Jan 2006 15:04:05")
		log.Printf("Setting system date to: %s\n", dateString)
		args := []string{"--set", dateString}
		return exec.Command("date", args...).Run()
	}
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
