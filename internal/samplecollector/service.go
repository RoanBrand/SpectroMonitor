package samplecollector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/RoanBrand/SpectroMonitor/cmd/sample-collector/config"
	"github.com/RoanBrand/SpectroMonitor/internal/db"
	"github.com/RoanBrand/SpectroMonitor/internal/model"
	"github.com/kardianos/service"
)

type app struct {
	conf *config.Config
	dbs  *db.DBs
	api  *http.Server

	ctx  context.Context
	stop context.CancelFunc
}

func New(c *config.Config) *app {
	return &app{conf: c}
}

func (a *app) Start(s service.Service) error {
	a.ctx, a.stop = signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	go a.startup()
	return nil
}

func (a *app) startup() {
	// DB retry until connect
	const retryTime = time.Second * 30
	for {
		dbs, err := db.New(a.ctx, a.conf.DBTestSamplesURL)
		if err != nil {
			log.Println("failed to connect to PostgreSQL:", err, ". Retrying in", retryTime)
			if sleepCtx(a.ctx, retryTime) {
				return
			}
		} else {
			a.dbs = dbs
			break
		}
	}

	a.doTask()
	go a.doTaskPeriodically()

	if err := a.setupAndStartAPIServer(); err != nil {
		panic(err)
	}
}

func (a *app) Stop(s service.Service) error {
	a.stop()
	err := a.stopAPIServer()
	a.dbs.Close()

	if err != nil {
		return fmt.Errorf("failed to stop http server: %w", err)
	}

	return nil
}

func (a *app) doTaskPeriodically() {
	interval := time.Duration(a.conf.RequestIntervalSeconds) * time.Second
	t := time.NewTimer(interval)

	for {
		select {
		case <-t.C:
			a.doTask()
			t.Reset(interval)
		case <-a.ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return
		}
	}
}

// get latest furnace results and update database on latest
func (a *app) doTask() {
	results, err := a.getResults()
	if err != nil {
		log.Println("failed getting results from API:", err)
		return
	}

	if err = a.dbs.ProcessResults(results); err != nil {
		log.Println("failed inserting results into DB:", err)
		return
	}
}

func (a *app) getResults() ([]model.Result, error) {
	resp, err := http.Get(a.conf.ResultsURL)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("request error: " + resp.Status)
	}

	defer resp.Body.Close()
	var res []model.Result

	if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res, nil
}

func (a *app) setupAndStartAPIServer() error {
	http.HandleFunc("/results", a.resultEndpoint)
	a.api.Addr = ":" + strconv.Itoa(a.conf.HTTPServerPort)
	err := a.api.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (a *app) stopAPIServer() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return a.api.Shutdown(ctx)
}

func (a *app) resultEndpoint(w http.ResponseWriter, r *http.Request) {
	results, err := a.dbs.GetLatest20ResultsForTVs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if len(results) == 0 {
		w.Write([]byte("[]"))
		return
	}

	if err = json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// return true if ctx cancelled or expired.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(d):
		return false
	}
}
