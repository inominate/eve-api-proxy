package main

import (
	"encoding/xml"
	"io/ioutil"
)

type configFile struct {
	Listen   string
	Threads  int
	Workers  int
	LogFile  string
	CacheDir string
}

var conf = loadConfig()

var defaultConfig = configFile{
	Listen:   "127.0.0.1:3748",
	Threads:  0,
	Workers:  10,
	LogFile:  "",
	CacheDir: "",
}

func createConfig() configFile {
	conf, _ := xml.MarshalIndent(defaultConfig, "", "  ")
	ioutil.WriteFile("eveapiproxy.xml", conf, 0644)
	return defaultConfig
}

func loadConfig() configFile {
	conf, err := ioutil.ReadFile("apiproxy.xml")
	if err != nil {
		return createConfig()
	}

	var newConfig configFile
	err = xml.Unmarshal(conf, &newConfig)
	if err != nil {
		return createConfig()
	}

	return newConfig
}
