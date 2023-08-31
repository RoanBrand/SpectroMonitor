package spectromon

import (
	"github.com/RoanBrand/SpectroMonitor/config"
	"github.com/RoanBrand/SpectroMonitor/displayboard"
	"github.com/RoanBrand/SpectroMonitor/http"
	"github.com/RoanBrand/SpectroMonitor/log"
	"github.com/kardianos/service"
)

type app struct {
	conf *config.Config
}

func New(c *config.Config) *app {
	return &app{conf: c}
}

func (a *app) Start(s service.Service) error {
	go a.startup()
	return nil
}

func (a *app) startup() {
	log.Setup(a.conf.LogFilePath, !service.Interactive())

	if err := displayboard.Start(a.conf.SerialPortName, a.conf.SerialBaudRate); err != nil {
		log.Fatal("could not open serial connection:", err)
	}

	http.Start(a.conf)
}

func (a *app) Stop(s service.Service) error {
	return nil
}
