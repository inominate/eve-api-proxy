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
	apiReq  *apicache.Request
	apiResp *apicache.Response

	data   []byte
	apiErr apicache.APIError

	expires time.Time

	worker   int
	httpCode int
	err      error
	respChan chan apiReq
}

var workChan chan apiReq

var apiClient apicache.Client
var activeWorkerCount, workerCount int32

func APIReq(url string, params map[string]string) ([]byte, int, error) {
	var errorStr string

	if atomic.LoadInt32(&workerCount) <= 0 {
		panic("No workers!")
	}
	useLog := atomic.LoadInt32(&logActive)

	// Build the request
	apireq := apicache.NewRequest(url)
	logParams := ""
	var paramVal string
	for k, v := range params {
		apireq.Set(k, v)
		// Show full vcode for log level 3
		if strings.ToLower(k) == "vcode" && useLog < 3 {
			paramVal = params[k][0:8] + "..."
		} else {
			paramVal = params[k]
		}
		logParams = fmt.Sprintf("%s&%s=%s", logParams, k, paramVal)
	}
	if logParams != "" {
		logParams = "?" + logParams[1:]
	}

	workerID := "C"
	// Don't send it to a worker if we can just yank it fromm the cache
	apiResp, err := apireq.GetCached()
	if err != nil {
		respChan := make(chan apiReq)
		req := apiReq{apiReq: apireq, respChan: respChan}
		workChan <- req

		resp := <-respChan
		close(respChan)

		apiResp = resp.apiResp
		err = resp.err
		workerID = fmt.Sprintf("%d", resp.worker)
	}

	if useLog > 0 && apiResp.Error.ErrorCode != 0 {
		errorStr = fmt.Sprintf(" Error %d: %s", apiResp.Error.ErrorCode, apiResp.Error.ErrorText)
	}
	if useLog != 0 || apiResp.HTTPCode != 200 {
		log.Printf("w%s: %s%s FromCache: %v HTTP: %d Expires: %s%s", workerID, url, logParams, apiResp.FromCache, apiResp.HTTPCode, apiResp.Expires.Format("2006-01-02 15:04:05"), errorStr)
	}

	return apiResp.Data, apiResp.HTTPCode, err
}

var logActive int32

func worker(reqChan chan apiReq, workerID int) {
	time.Sleep(time.Duration(workerID) * time.Second)

	atomic.AddInt32(&workerCount, 1)
	for req := range reqChan {
		atomic.AddInt32(&activeWorkerCount, 1)

		resp, err := req.apiReq.Do()
		req.apiResp = resp
		req.err = err

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
