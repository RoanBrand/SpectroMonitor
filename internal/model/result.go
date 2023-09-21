package model

import "time"

type elementResult struct {
	Element string  `json:"element"`
	Value   float64 `json:"value"`
}

type Result struct {
	SampleName string          `json:"sample_name"`
	Furnace    string          `json:"furnace"`
	TimeStamp  time.Time       `json:"time_stamp"`
	Results    []elementResult `json:"results,omitempty"`

	Spectro int `json:"Spectro"` // spectro machine from which the sample was taken
}
