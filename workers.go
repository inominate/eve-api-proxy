package main

import (
	"fmt"
	"github.com/inominate/eve-api-proxy/apicache"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var apiClient apicache.Client

// worker tracking
var activeWorkerCount, workerCount int32
var workCount []int32

type apiReq struct {
	apiReq  *apicache.Request
	apiResp *apicache.Response

	apiErr apicache.APIError

	expires time.Time

	worker   int
	httpCode int
	err      error
	respChan chan apiReq
}

// Channel for sending jobs to workers
var workChan chan apiReq

func APIReq(url string, params map[string]string) (*apicache.Response, error) {
	var errorStr string

	if atomic.LoadInt32(&workerCount) <= 0 {
		panic("No workers!")
	}

	// Build the request
	apireq := apicache.NewRequest(url)
	for k, v := range params {
		switch k {
		case "force":
			if v != "" {
				apireq.Force = true
			}
		default:
			apireq.Set(k, v)
		}
	}

	workerID := "C"
	// Don't send it to a worker if we can just yank it fromm the cache
	apiResp, err := apireq.GetCached()
	if err != nil || apireq.Force {
		respChan := make(chan apiReq)
		req := apiReq{apiReq: apireq, respChan: respChan}
		workChan <- req

		resp := <-respChan
		close(respChan)

		apiResp = resp.apiResp
		err = resp.err
		workerID = fmt.Sprintf("%d", resp.worker)
	}

	// This is similar to the request log, but knows more about where it came from.
	if debug {
		if apiResp.Error.ErrorCode != 0 {
			errorStr = fmt.Sprintf(" Error %d: %s", apiResp.Error.ErrorCode, apiResp.Error.ErrorText)
		}
		logParams := ""
		var paramVal string
		for k, _ := range params {
			if conf.Logging.CensorLog && strings.ToLower(k) == "vcode" && len(params[k]) > 8 {
				paramVal = params[k][0:8] + "..."
			} else {
				paramVal = params[k]
			}
			logParams = fmt.Sprintf("%s&%s=%s", logParams, k, paramVal)
		}
		if logParams != "" {
			logParams = "?" + logParams[1:]
		}

		debugLog.Printf("w%s: %s%s HTTP: %d Expires: %s%s", workerID, url, logParams, apiResp.HTTPCode, apiResp.Expires.Format("2006-01-02 15:04:05"), errorStr)
	}
	return apiResp, err
}

func worker(reqChan chan apiReq, workerID int) {
	atomic.AddInt32(&workerCount, 1)
	for req := range reqChan {
		atomic.AddInt32(&activeWorkerCount, 1)

		resp, err := req.apiReq.Do()
		req.apiResp = resp
		req.err = err
		req.worker = workerID

		req.respChan <- req
		atomic.AddInt32(&workCount[workerID], 1)
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
	workCount = make([]int32, conf.Workers)

	for i := 1; i <= conf.Workers; i++ {
		debugLog.Printf("Starting worker #%d.", i)
		go worker(workChan, i)
	}
}

func stopWorkers() {
	close(workChan)
	for atomic.LoadInt32(&workerCount) > 0 {
		time.Sleep(10 * time.Millisecond)
	}
	startWorkersOnce = &sync.Once{}
}

func PrintWorkerStats() {
	active, loaded := GetWorkerStats()
	log.Printf("%d workers idle, %d workers active.", loaded-active, active)

	for i := int32(0); i < atomic.LoadInt32(&workerCount); i++ {
		count := atomic.LoadInt32(&workCount[i])
		log.Printf("   %d: %d", i, count)
	}
}

func GetWorkerStats() (int32, int32) {
	return atomic.LoadInt32(&activeWorkerCount), atomic.LoadInt32(&workerCount)
}
