package model

import "time"

type ElementResult struct {
	Element string  `json:"element"`
	Value   float64 `json:"value"`
}

// For TVs
type Result struct {
	SampleName string          `json:"sample_name"`
	Furnace    string          `json:"furnace"`
	TimeStamp  time.Time       `json:"time_stamp"`
	Results    []ElementResult `json:"results,omitempty"`

	Spectro int `json:"Spectro"` // spectro machine from which the sample was taken
}
