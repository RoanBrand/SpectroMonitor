package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RoanBrand/SpectroMonitor/config"
	"github.com/RoanBrand/SpectroMonitor/log"
	"github.com/RoanBrand/SpectroMonitor/spectromon"

	"github.com/kardianos/service"
)

const usageMsg = "Specify config -c=config.json"

func main() {
	svcFlag := flag.String("service", "", "Control the system service.")
	confFlag := flag.String("c", "", usageMsg)
	flag.Parse()

	if *confFlag == "" {
		log.Fatal(usageMsg)
	}

	conf, err := config.LoadConfig(*confFlag)
	if err != nil {
		log.Fatal("error parsing config '"+*confFlag+"': ", err)
	}

	gracefulStop := make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		log.Printf("Caught sig: %+v", sig)
		log.Println("Wait for 4 seconds to finish processing")

		time.Sleep(4 * time.Second)
		os.Exit(0)
	}()

	svcConfig := &service.Config{
		Name:        "SpectroMonitor",
		DisplayName: "Spectrometer Alert App",
		Description: "Powers light indications & time display boards",
	}

	s, err := service.New(spectromon.New(conf), svcConfig)
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
