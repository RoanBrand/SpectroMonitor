package config

import (
	"encoding/json"
	"os"
	"path"
)

type Config struct {
	DBTestSamplesURL string `json:"db_test_samples_url"`
	ResultsURL       string `json:"results_url"`

	HTTPServerPort int `json:"http_server_port"`

	RequestIntervalSeconds int `json:"request_interval_seconds"` // time between requests
}

func LoadConfig(filePath string) (*Config, error) {
	// if file not found in current WD, try executable's folder
	if !fileExists(filePath) {
		exePath, err := os.Executable()
		if err != nil {
			return nil, err
		}
		filePath = path.Join(path.Dir(exePath), filePath)
	}

	conf := &Config{
		HTTPServerPort:         80,
		RequestIntervalSeconds: 10}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	if err = json.NewDecoder(f).Decode(conf); err != nil {
		return nil, err
	}

	return conf, nil
}

// fileExists checks if a file exists and is not a directory before we try using it to prevent further errors.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
