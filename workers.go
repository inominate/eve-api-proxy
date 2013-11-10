package main

import (
	"fmt"
	"ieveapi/apicache"
	"log"
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

func worker(reqChan chan apiReq) {
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
		log.Printf("%s - %+v FromCache: %v HTTP: %d%s", req.url, req.params, resp.FromCache, resp.HTTPCode, errorStr)

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
		go worker(workChan)
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

func GetWorkerStats() (int32, int32) {
	return atomic.LoadInt32(&activeWorkerCount), atomic.LoadInt32(&workerCount)
}
