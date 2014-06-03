package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/inominate/apicache"
)

type APIMux struct{}

func makeParams(req *http.Request) map[string]string {
	params := make(map[string]string)
	for key, val := range req.Form {
		params[key] = val[0]
	}

	return params
}

func logRequest(req *http.Request, url string, params map[string]string, resp *apicache.Response, startTime time.Time) {
	remoteAddr, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		remoteAddr = req.RemoteAddr
	}
	// Should we use a different header for our real address?
	if conf.RealRemoteAddrHeader != "" && req.Header.Get(conf.RealRemoteAddrHeader) != "" {
		if conf.ProxyAddr == "" || remoteAddr == conf.ProxyAddr {
			remoteAddr = req.Header.Get(conf.RealRemoteAddrHeader)
		}
	}

	if resp == nil {
		if conf.Logging.LogRequests && !debug {
			log.Printf("%s - Invalid Request for %s", remoteAddr, url)
		}
		debugLog.Printf("%s - Invalid Request for %s - %+v", remoteAddr, url, req)
		return
	}

	var errorStr string
	if resp.Error.ErrorCode != 0 {
		errorStr = fmt.Sprintf("Error %d: %s", resp.Error.ErrorCode, resp.Error.ErrorText)
	}

	logParams := ""
	var paramVal string
	for k, _ := range params {
		// vCode censorship
		if conf.Logging.CensorLog && strings.ToLower(k) == "vcode" {
			paramVal = params[k][0:8] + "..."
		} else {
			paramVal = params[k]
		}
		logParams = fmt.Sprintf("%s&%s=%s", logParams, k, paramVal)
	}

	if logParams != "" {
		logParams = "?" + logParams[1:]
	}
	log.Printf("%s - %s%s - http: %d - expires: %s - %.2f seconds - %s",
		remoteAddr, url, logParams, resp.HTTPCode,
		resp.Expires.Format("2006-01-02 15:04:05"), time.Since(startTime).Seconds(),
		errorStr)
}

// The muxer for the whole operation.  Everything starts here.
func (a APIMux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var resp *apicache.Response
	startTime := time.Now()

	req.ParseForm()

	url := path.Clean(req.URL.Path)
	if url == "/stats" {
		statsHandler(w, req)
		return
	}

	params := makeParams(req)

	debugLog.Printf("Starting request for %s...", url)

	w.Header().Add("Content-Type", "text/xml")
	if handler, valid := validPages[strings.ToLower(url)]; valid {
		if handler == nil {
			handler = defaultHandler
		}

		resp = handler(url, params)

		w.WriteHeader(resp.HTTPCode)
		w.Write(resp.Data)
	} else {
		w.WriteHeader(404)
		w.Write(apicache.SynthesizeAPIError(404, "Invalid API page.", 24*time.Hour))
	}

	if conf.Logging.LogRequests || (resp != nil && resp.HTTPCode != 200) {
		logRequest(req, url, params, resp, startTime)
	}

	if debug && time.Since(startTime).Seconds() > 10 {
		debugLog.Printf("Slow Request took %.2f seconds:", time.Since(startTime).Seconds())
		debugLog.Printf("%+v", req)
	}
}

func statsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	LogStats(w)
}

func LogStats(w io.Writer) {
	PrintWorkerStats(w)
	fmt.Fprintln(w, "")
	dc.LogStats(w)
	fmt.Fprintln(w, "")
	LogMemStats(w)
}

func LogMemStats(w io.Writer) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Fprintf(w, "Alloc: %dkb Sys: %dkb\n", m.Alloc/1024, m.Sys/1024)
	fmt.Fprintf(w, "HeapAlloc: %dkb HeapSys: %dkb\n", m.HeapAlloc/1024, m.HeapSys/1024)
}
