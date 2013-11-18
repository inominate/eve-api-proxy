package main

import (
	"apiproxy/apicache"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type apiReq struct {
	url    string
	params map[string]string
	data   []byte
	apiErr apicache.APIError

	expires time.Time

	httpCode int
	err      error
	respChan chan apiReq
}

var workChan chan apiReq

var apiClient apicache.Client
var activeWorkerCount, workerCount int32

func APIReq(url string, params map[string]string) ([]byte, int, error) {
	if atomic.LoadInt32(&workerCount) <= 0 {
		startWorkers()
	}

	respChan := make(chan apiReq)
	req := apiReq{url: url, params: params, respChan: respChan}
	workChan <- req

	resp := <-respChan
	close(respChan)

	return resp.data, resp.httpCode, resp.err
}

var logActive int32

func worker(reqChan chan apiReq, workerID int) {
	time.Sleep(time.Duration(workerID) * time.Second)

	atomic.AddInt32(&workerCount, 1)
	for req := range reqChan {
		atomic.AddInt32(&activeWorkerCount, 1)
		apireq := apicache.NewRequest(req.url)
		for k, v := range req.params {
			apireq.Set(k, v)
		}

		resp, err := apireq.Do()
		req.data = resp.Data
		req.expires = resp.Expires
		req.apiErr = resp.Error
		req.httpCode = resp.HTTPCode
		req.err = err

		var errorStr string
		if resp.Error.ErrorCode != 0 {
			errorStr = fmt.Sprintf(" Error %d: %s", resp.Error.ErrorCode, resp.Error.ErrorText)
		}
		useLog := atomic.LoadInt32(&logActive)
		if useLog != 0 || resp.HTTPCode != 200 {
			logParams := map[string]string{}
			for k, _ := range req.params {
				if strings.ToLower(k) == "vcode" {
					logParams[k] = req.params[k][0:8] + "..."
				} else {
					logParams[k] = req.params[k]
				}
			}
			log.Printf("w%d: %s - %+v FromCache: %v HTTP: %d Expires: %s%s", workerID, req.url, logParams, resp.FromCache, resp.HTTPCode, resp.Expires.Format("2006-01-02 15:04:05"), errorStr)
		}

		req.respChan <- req
		atomic.AddInt32(&activeWorkerCount, -1)
	}
	atomic.AddInt32(&workerCount, -1)
}

var startWorkersOnce = &sync.Once{}

func startWorkers() {
	startWorkersOnce.Do(realStartWorkers)
}

func realStartWorkers() {
	log.Printf("Starting %d Workers...", conf.Workers)
	workChan = make(chan apiReq)

	for i := 0; i < conf.Workers; i++ {
		go worker(workChan, i)
	}
}

func stopWorkers() {
	close(workChan)
	for atomic.LoadInt32(&activeWorkerCount) > 0 {
		time.Sleep(10 * time.Millisecond)
	}
	startWorkersOnce = &sync.Once{}
}

func PrintWorkerStats() {
	active, loaded := GetWorkerStats()
	log.Printf("%d workers idle, %d workers active.", loaded-active, active)
}

func EnableVerboseLogging() int32 {
	newLog := atomic.AddInt32(&logActive, 1)
	return newLog
}

func DisableVerboseLogging() {
	useLog := atomic.LoadInt32(&logActive)
	atomic.AddInt32(&logActive, -useLog)
}

func GetWorkerStats() (int32, int32) {
	return atomic.LoadInt32(&activeWorkerCount), atomic.LoadInt32(&workerCount)
}
