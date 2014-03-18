package main

import (
	"crypto/rand"
	"encoding/xml"
	"io"
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
	Secret     string
}

var conf = loadConfig()

func genSecret() string {
	buf := make([]byte, 32)
	io.ReadFull(rand.Reader, buf)
	return fmt.Sprintf("%x", buf)
}

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
	conf.Secret = genSecret()

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
