package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/inominate/apicache"
	"github.com/inominate/eve-api-proxy/ratelimit"
)

var debugLog *log.Logger
var debug bool

var dc *DiskCache

var rateLimiter *ratelimit.RateLimit
var errorRateLimiter *ratelimit.RateLimit

func main() {
	var err error
	log.SetFlags(0)

	conf, err = loadConfig("apiproxy.xml")

	var newConfig bool
	// Check and process command line flags
	flag.BoolVar(&newConfig, "create", false, "Create new config file from detaults.")
	flag.BoolVar(&conf.Logging.Debug, "debug", conf.Logging.Debug, "Enable debug logging.")
	flag.BoolVar(&conf.FastStart, "q", conf.FastStart, "Fast startup, delete existing cache instead of loading it.")
	flag.Parse()
	//////////////////////////////////////

	if newConfig {
		createConfig()
	}

	if err != nil {
		log.Fatalf("Error loading configuration: %s", err)
	}

	debug = conf.Logging.Debug
	setupLogging()

	// We do everything in UTC/evetime.
	time.Local = time.UTC

	// Set up max threads, this will likely go away in a future version of Go
	if conf.Threads == 0 {
		conf.Threads = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(conf.Threads)
	log.Printf("EVEAPIProxy Starting Up with %d threads...", conf.Threads)
	//////////////////////////////////////

	// Initialize and configure the apicache module.
	log.Printf("Initializing Disk Cache...")
	dc = NewDiskCache(conf.CacheDir, conf.FastStart)
	log.Printf("Done.")

	apicache.NewClient(dc)
	apicache.SetMaxIdleConns(conf.Workers)
	apicache.GetDefaultClient().Retries = conf.Retries
	apicache.GetDefaultClient().SetTimeout(time.Duration(conf.APITimeout) * time.Second)

	ua := "eve-api-proxy by Innominate - http://github.com/inominate/eve-api-proxy"
	if conf.UserAgent != "" {
		ua = conf.UserAgent
	}
	apicache.GetDefaultClient().UserAgent = ua
	//////////////////////////////////////

	errorRateLimiter = ratelimit.NewRateLimit(conf.MaxErrors, time.Duration(conf.ErrorPeriod)*time.Second)
	rateLimiter = ratelimit.NewRateLimit(conf.RequestsPerSecond, time.Second)

	startWorkers()

	// Fire up the http server
	var handler APIMux
	server := http.Server{
		Addr:         conf.Listen,
		Handler:      &handler,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
	}

	log.Fatal(server.ListenAndServe())
}

func setupLogging() {
	var logfp io.Writer
	var debugfp io.Writer
	var err error

	logfp = os.Stdout
	debugfp = ioutil.Discard
	logflag := log.Ldate | log.Ltime

	if conf.Logging.LogFile != "" {
		logfp, err = os.OpenFile(conf.Logging.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatalf("Cannot Open Log File: %s", err)
		}
	}
	log.SetOutput(logfp)

	if debug {
		if conf.Logging.DebugLogFile != conf.Logging.LogFile {
			if conf.Logging.DebugLogFile == "" {
				debugfp = os.Stdout
			} else {
				debugfp, err = os.OpenFile(conf.Logging.DebugLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
				if err != nil {
					log.Fatalf("Cannot Open Debug Log File: %s", err)
				}
			}
		} else {
			debugfp = logfp
		}
	}

	debugLog = log.New(debugfp, "DEBUG ", logflag)
	apicache.DebugLog = debugLog

	log.SetFlags(logflag)
	debugLog.SetFlags(logflag)
}
