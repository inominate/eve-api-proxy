package main

import (
	"apiproxy/apicache"
	"log"
	"net/http"
	"path"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

type APIMux struct{}

func (a APIMux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	startTime := time.Now()

	req.ParseForm()
	url := path.Clean(req.URL.Path)

	useLog := atomic.LoadInt32(&logActive)
	if useLog >= 5 {
		log.Printf("Starting request for %s...", url)
	}

	if url == "/stats" {
		LogStats()
		w.Write([]byte(""))
		return
	}

	if url == "/logon" {
		ll := EnableVerboseLogging()
		log.Printf("Logging verbosity increased to: %d", ll)
		w.Write([]byte(""))
		return
	}

	if url == "/logoff" {
		log.Printf("Verbose logging disabled")
		DisableVerboseLogging()
		w.Write([]byte(""))
		return
	}

	w.Header().Add("Content-Type", "text/xml")
	if handler, valid := validPages[strings.ToLower(url)]; valid {
		if handler == nil {
			handler = defaultHandler
		}

		handler(w, req)
	} else {
		log.Printf("Invalid URL %s - %s", url, req.Form)
		w.WriteHeader(404)
		w.Write(apicache.SynthesizeAPIError(404, "Invalid API page.", 24*time.Hour))
	}

	if useLog >= 4 {
		log.Printf("Request took: %.2f seconds.", time.Since(startTime).Seconds())
	} else if useLog >= 1 && time.Since(startTime).Seconds() > 10 {
		log.Printf("Slow Request took %.2f seconds:", time.Since(startTime).Seconds())
		log.Printf("Request for %s: %+v", url, req.Form)
	}
}

func LogStats() {
	PrintWorkerStats()
	dc.LogStats()
	LogMemStats()
}

func LogMemStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("Alloc: %dkb Sys: %dkb", m.Alloc/1024, m.Sys/1024)
	log.Printf("HeapAlloc: %dkb HeapSys: %dkb", m.HeapAlloc/1024, m.HeapSys/1024)
}
