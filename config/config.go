package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	ModbusURL          string `json:"modbus_url"`
	ModbusAddrLights   uint16 `json:"modbus_address_start_lights"`
	ModbusAddrDisplays uint16 `json:"modbus_address_start_displays"`

	SerialPortName string `json:"serial_port_name"`
	SerialBaudRate int    `json:"serial_baud_rate"`

	LogFilePath string `json:"log_file_path"`

	TransferSamplesOnly           bool      `json:"transfer_samples_only"`
	ResultUrl                     string    `json:"result_url"`
	RequestIntervalSeconds        int       `json:"request_interval_seconds"` // time between requests
	DisplayBoardUpdateRateSeconds int       `json:"display_board_update_rate_seconds"`
	TimeUpdateUrl                 string    `json:"time_update_url"`
	TimeUpdateIntervalSeconds     int       `json:"time_update_interval_seconds"`
	FurnaceResultOldTimeMinutes   int       `json:"furnace_result_old_time_minutes"` // time in minutes after sample is old
	Furnaces                      []furnace `json:"furnaces"`
}

type furnace struct {
	Name string `json:"name"`

	/*LightCardAddress  uint8 `json:"light_card_address"`
	GreenLightAddress uint8 `json:"green_light_address"`
	RedLightAddress   uint8 `json:"red_light_address"`*/

	DisplayBoardAddress uint8 `json:"display_board_address"`
}

func LoadConfig(filePath string) (*Config, error) {
	conf := Config{
		SerialBaudRate:                9600,
		SerialPortName:                "com7",
		ResultUrl:                     "localhost/lastfurnaceresults",
		RequestIntervalSeconds:        60 * 60,
		DisplayBoardUpdateRateSeconds: 1,
		TimeUpdateIntervalSeconds:     60 * 5,
		FurnaceResultOldTimeMinutes:   120,
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(f)
	err = dec.Decode(&conf)
	if err != nil {
		return nil, err
	}

	return &conf, nil
}
