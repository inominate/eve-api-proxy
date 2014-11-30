package main

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inominate/apicache"
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
		for i := 0; i < conf.Retries; i++ {
			respChan := make(chan apiReq)
			req := apiReq{apiReq: apireq, respChan: respChan}
			workChan <- req

			resp := <-respChan
			close(respChan)

			apiResp = resp.apiResp
			err = resp.err
			workerID = fmt.Sprintf("%d", resp.worker)

			// Attempt to recover from server issues, invalidate flag means we
			// believe this is not a server failure.
			// 418 is the tempban code
			// 500/900 are panic codes
			if err == nil || apiResp.Invalidate || apiResp.HTTPCode == 418 || apiResp.HTTPCode == 500 || apiResp.HTTPCode == 900 {
				break
			}
			time.Sleep(2 * time.Second)
			apireq.Force = true
		}
	}

	// I HATE 221 HATE HATE HAAAAAAAATE
	var singleDebug bool
	if apiResp.Error.ErrorCode == 221 {
		singleDebug = true
	}

	// This is similar to the request log, but knows more about where it came from.
	if debug || singleDebug {
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
		var err, eErr, rErr error
		var errStr string

		atomic.AddInt32(&activeWorkerCount, 1)

		// Run both of the error limiters simultaneously rather than in
		// sequence. Still need both before we continue.
		errorLimiter := make(chan error)
		rpsLimiter := make(chan error)
		go func() {
			err := errorRateLimiter.Start(30 * time.Second)
			errorLimiter <- err
		}()
		go func() {
			err := rateLimiter.Start(30 * time.Second)
			rpsLimiter <- err
		}()
		eErr = <-errorLimiter
		rErr = <-rpsLimiter

		// Check the error limiter for timeouts
		if eErr != nil {
			err = eErr
			errStr = "error throttling"

			// If the rate limiter didn't timeout be sure to signal it that we
			// didn't do anything.
			if rErr == nil {
				rateLimiter.Finish(true)
			}
		}
		if rErr != nil {
			err = rErr
			if errStr == "" {
				errStr = "rate limiting"
			} else {
				errStr += " and rate limiting"
			}

			// If the error limiter didn't also timeout be sure to signal it that we
			// didn't do anything.
			if eErr == nil {
				errorRateLimiter.Finish(true)
			}
		}
		// We're left with a single err and errStr for returning an error to the client.
		if err != nil {
			log.Printf("Rate Limit Error: %s - %s", errStr, err)
			log.Printf("RPS Events: %d Outstanding: %d", rateLimiter.Count(), rateLimiter.Outstanding())
			log.Printf("Errors Events: %d Outstanding: %d", errorRateLimiter.Count(), errorRateLimiter.Outstanding())

			req.apiResp = &apicache.Response{
				Data: apicache.SynthesizeAPIError(500,
					fmt.Sprintf("APIProxy Error: Proxy timeout due to %s.", errStr),
					5*time.Minute),
				Expires: time.Now().Add(5 * time.Minute),
				Error: apicache.APIError{500,
					fmt.Sprintf("APIProxy Error: Proxy timeout due to %s.", errStr)},
				HTTPCode: 504,
			}
			req.err = err
		} else {
			resp, err := req.apiReq.Do()
			req.apiResp = resp
			req.err = err
			if resp.Error.ErrorCode == 0 || resp.HTTPCode == 504 || resp.HTTPCode == 418 {
				// 418 means we are currently tempbanned from the API.
				// 504 means the API proxy had some kind of internal or network error.
				//
				// We do not treat these as an error for rate limiting because
				// the apicache library handles it for us, these requests are
				// not actually making it to the CCP API.

				// Finish, but skip recording the event in the rate limiter
				// when there is no error.
				errorRateLimiter.Finish(true)
			} else {
				errorRateLimiter.Finish(false)
			}
			rateLimiter.Finish(false)
		}

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
	workCount = make([]int32, conf.Workers+1)

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

func PrintWorkerStats(w io.Writer) {
	active, loaded := GetWorkerStats()
	fmt.Fprintf(w, "%d workers idle, %d workers active.\n", loaded-active, active)

	rateCount := rateLimiter.Count()
	rateOutstanding := rateLimiter.Outstanding()

	errorCount := errorRateLimiter.Count()
	errorOutstanding := errorRateLimiter.Outstanding()

	fmt.Fprintf(w, "%d requests in the last second. %d requests outstanding.\n", rateCount, rateOutstanding)
	fmt.Fprintf(w, "%d errors over last %d seconds. %d errors outstanding.\n", errorCount, conf.ErrorPeriod, errorOutstanding)

	for i := int32(1); i <= atomic.LoadInt32(&workerCount); i++ {
		count := atomic.LoadInt32(&workCount[i])
		fmt.Fprintf(w, "   %d: %d\n", i, count)
	}
}

func GetWorkerStats() (int32, int32) {
	return atomic.LoadInt32(&activeWorkerCount), atomic.LoadInt32(&workerCount)
}
