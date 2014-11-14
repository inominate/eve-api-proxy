package main

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
)

type configFile struct {
	Listen string

	Threads int
	Workers int

	Retries    int
	APITimeout int

	RequestsPerSecond int
	ErrorPeriod       int
	MaxErrors         int

	CacheDir  string
	FastStart bool

	Secret               string `xml:",omitempty"`
	ProxyAddr            string `xml:",omitempty"`
	RealRemoteAddrHeader string `xml:",omitempty"`
	UserAgent            string `xml:",omitempty"`

	Logging logConfig
}

type logConfig struct {
	LogFile string

	LogRequests bool
	CensorLog   bool

	Debug        bool
	DebugLogFile string
}

var conf configFile

func genSecret() string {
	buf := make([]byte, 32)
	io.ReadFull(rand.Reader, buf)
	return fmt.Sprintf("%x", buf)
}

var defaultConfig = configFile{
	Listen:  "127.0.0.1:3748",
	Threads: 0,
	Workers: 10,

	RequestsPerSecond: 30,

	ErrorPeriod: 60,
	MaxErrors:   75,

	Retries:    3,
	APITimeout: 60,

	CacheDir: "cache/",
	Logging: logConfig{
		CensorLog: true,
	},
}

func createConfig() {
	//  Secret for eventual control code
	//	defaultConfig.Secret = genSecret()
	confXML, _ := xml.MarshalIndent(defaultConfig, "", "  ")

	err := ioutil.WriteFile("apiproxy.xml.default", confXML, 0600)
	if err != nil {
		log.Fatalf("Error creating config file apiproxy.xml.default: %s", err)
	} else {
		log.Fatalf("Created new config file apiproxy.xml.default")
	}
}

func loadConfig(filename string) (configFile, error) {
	conf, err := ioutil.ReadFile(filename)
	if err != nil {
		return defaultConfig, err
	}

	newConfig := defaultConfig
	err = xml.Unmarshal(conf, &newConfig)
	if err != nil {
		return defaultConfig, err
	}

	if newConfig.CacheDir == "" {
		return defaultConfig, fmt.Errorf("Need cache directory")
	}

	if newConfig.Logging.Debug {
		debug = true
	}

	return newConfig, nil
}
