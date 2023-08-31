package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/RoanBrand/SpectroMonitor/config"
)

type resultResponse struct {
	SampleName string    `json:"sample_name"`
	Furnace    string    `json:"furnace"`
	TimeStamp  time.Time `json:"time_stamp"`
}

func GetResult(url string, conf *config.Config) ([]resultResponse, error) {
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

func GetTime(url string) (time.Time, error) {
	res := struct {
		T time.Time `json:"t"`
	}{}

	resp, err := http.Get(url)
	if err != nil {
		return res.T, err
	}

	if resp.StatusCode != http.StatusOK {
		return res.T, errors.New("gettime request error: " + resp.Status)
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&res)
	return res.T, err
}
