package main

import (
	"encoding/xml"
	"io/ioutil"
)

type configFile struct {
	Listen     string
	Threads    int
	Workers    int
	Retries    int
	APITimeout int
	LogFile    string
	CacheDir   string
}

var conf = loadConfig()

var defaultConfig = configFile{
	Listen:     "127.0.0.1:3748",
	Threads:    0,
	Workers:    10,
	Retries:    3,
	APITimeout: 60,
	LogFile:    "",
	CacheDir:   "",
}

func createConfig() configFile {
	conf, _ := xml.MarshalIndent(defaultConfig, "", "  ")
	ioutil.WriteFile("apiproxy.xml", conf, 0644)
	return defaultConfig
}

func loadConfig() configFile {
	conf, err := ioutil.ReadFile("apiproxy.xml")
	if err != nil {
		return createConfig()
	}

	newConfig := defaultConfig
	err = xml.Unmarshal(conf, &newConfig)
	if err != nil {
		return createConfig()
	}

	if newConfig.CacheDir == "" {
		panic("Need cache directory")
	}
	return newConfig
}
