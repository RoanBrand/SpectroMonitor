package main

import (
	"flag"
	"log"

	"github.com/RoanBrand/SpectroMonitor/cmd/sample-collector/config"
	"github.com/RoanBrand/SpectroMonitor/internal/samplecollector"
	"github.com/kardianos/service"
)

func main() {
	svcFlag := flag.String("service", "", "Control the system service.")
	confFlag := flag.String("c", "sample-collector-config.json", "Specify config -c=sample-collector-config.json")
	flag.Parse()

	conf, err := config.LoadConfig(*confFlag)
	if err != nil {
		log.Fatal("error parsing config '"+*confFlag+"': ", err)
	}

	svcConfig := &service.Config{
		Name:        "SampleCollector",
		DisplayName: "Spectro Test Samples Data Collector",
		Description: "Collects data for central DB for TV and Alerts",
	}

	s, err := service.New(samplecollector.New(conf), svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	if *svcFlag != "" {
		err = service.Control(s, *svcFlag)
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}
		return
	}

	logger, err := s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}
