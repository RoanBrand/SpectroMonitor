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
	"path/filepath"
	"sort"
	"strconv"
	"sync"
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

	ctx  context.Context
	stop context.CancelFunc

	api http.Server

	// TV API result cache
	cacheLock    sync.RWMutex
	cacheExpires time.Time
	cacheResult  []byte
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
	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}

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

	websiteDir := filepath.Join(filepath.Dir(exePath), "website")

	if err := a.setupAndStartAPIServer(websiteDir); err != nil {
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
	var l sync.Mutex
	var wg sync.WaitGroup
	results := make([]model.ResultXML, 0, len(a.conf.Spectros)*20)

	wg.Add(len(a.conf.Spectros))

	for _, spectro := range a.conf.Spectros {
		go func(spectro *config.SpectroMachine) {
			defer wg.Done()

			sRes, err := a.getResults(spectro.ResultsURL)
			if err != nil {
				log.Printf("failed getting results from spectro %d API %s: %v", spectro.Number, spectro.ResultsURL, err)
				return
			}

			for i := range sRes {
				sRes[i].Spectro = spectro.Number
			}

			l.Lock()
			results = append(results, sRes...)
			l.Unlock()
		}(spectro)
	}

	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].TimeStamp.After(results[j].TimeStamp)
	})

	if err := a.dbs.ProcessResults(results); err != nil {
		log.Println("failed inserting results into DB:", err)
		return
	}
}

func (a *app) getResults(url string) ([]model.ResultXML, error) {
	ctx, cancel := context.WithTimeout(a.ctx, time.Second*10)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var c http.Client
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("request error: " + resp.Status)
	}

	defer resp.Body.Close()
	var res []model.ResultXML

	if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res, nil
}

func (a *app) setupAndStartAPIServer(websiteFilesPath string) error {
	http.Handle("/", http.FileServer(http.Dir(websiteFilesPath)))
	http.HandleFunc("/results", a.resultEndpoint)
	http.HandleFunc("/lastfurnaceresults", a.lastFurnaceResultEndpoint)
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
	results, err := a.getAPIResultsCache()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.Write(results)
}

func (a *app) lastFurnaceResultEndpoint(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	results, err := a.dbs.GetLatestResultsOfFurnaces(q["f"])
	if err != nil {
		log.Println("error GetLatestResultsOfFurnaces: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(results)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *app) getAPIResultsCache() ([]byte, error) {
	a.cacheLock.RLock()
	if time.Now().Before(a.cacheExpires) {
		defer a.cacheLock.RUnlock()
		return a.cacheResult, nil
	}

	a.cacheLock.RUnlock()
	a.cacheLock.Lock()
	defer a.cacheLock.Unlock()

	if time.Now().Before(a.cacheExpires) {
		return a.cacheResult, nil
	}

	results, err := a.dbs.GetLatest20ResultsForTVs()
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		a.cacheResult = []byte("[]")
	} else {
		resJson, err := json.Marshal(results)
		if err != nil {
			return nil, err
		}
		a.cacheResult = resJson
	}

	a.cacheExpires = time.Now().Add(time.Second * 5)
	return a.cacheResult, nil
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
