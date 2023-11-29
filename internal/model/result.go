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

// From Spectros
// TODO: use similar struct as defacto type between spectros and sample-collector.
// Also try and send it on to website as is, and have it reorder and decide what to display.
type ResultXML struct {
	ID        string             `json:"id"`
	Furnace   string             `json:"furnace"`
	TimeStamp time.Time          `json:"time_stamp"`
	Results   map[string]float64 `json:"results"`

	Spectro int `json:"Spectro"` // spectro machine from which the sample was taken
}
