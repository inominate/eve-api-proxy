package main

import (
	"apiproxy/apicache"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"
)

var dc *DiskCache
var clearCache = flag.Bool("clear", false, "Delete existing cache instead of loading it.")

func main() {
	flag.Parse()

	time.Local = time.UTC

	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
	if conf.LogFile != "" {
		logfp, err := os.OpenFile(conf.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(fmt.Sprintf("Cannot Open Log File: %s", err))
		}
		log.SetOutput(logfp)
	}

	if conf.Threads == 0 {
		conf.Threads = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(conf.Threads)
	log.Printf("EVEAPIProxy Starting Up with %d threads...", conf.Threads)

	log.Printf("Initializing Disk Cache...")
	dc = NewDiskCache(conf.CacheDir, *clearCache)
	log.Printf("Done.")

	apicache.NewClient(dc)
	//	apicache.GetDefaultClient().Retries = 5
	startWorkers()

	var handler APIHandler

	server := http.Server{
		Addr:         conf.Listen,
		Handler:      &handler,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
	}

	log.Fatal(server.ListenAndServe())
}
