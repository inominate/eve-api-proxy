package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

type cacheEntry struct {
	httpCode int
	expires  time.Time
}

type DiskCache struct {
	cacheRoot  string
	cacheFiles map[string]cacheEntry
	sync.RWMutex
}

func (d *DiskCache) check() {
	rootF, _ := os.Open(d.cacheRoot)

	d.Lock()
	defer d.Unlock()

	files, _ := rootF.Readdir(0)
	for _, fi := range files {
		_, exists := d.cacheFiles[fi.Name()]
		if !exists {
			os.Remove(d.cacheRoot + "/" + fi.Name())
		}
	}
}

var cleanOnce = &sync.Once{}

func (d *DiskCache) clean() {
	now := time.Now()
	for tag, ce := range d.cacheFiles {
		if now.After(ce.expires) {
			os.Remove(d.cacheRoot + "/" + tag)
			delete(d.cacheFiles, tag)
		}
	}
	cleanOnce = &sync.Once{}
}

var storeCount = 0

func (d *DiskCache) Store(cacheTag string, httpCode int, data []byte, expires time.Time) error {
	d.Lock()
	defer d.Unlock()

	storeCount++
	if storeCount >= 50 {
		go cleanOnce.Do(func() { d.clean() })
		storeCount = 0
	}

	ce := cacheEntry{httpCode, expires}
	err := ioutil.WriteFile(d.cacheRoot+"/"+cacheTag, data, 0600)
	if err != nil {
		return err
	}

	d.cacheFiles[cacheTag] = ce
	return nil
}

func (d *DiskCache) Get(cacheTag string) (int, []byte, time.Time, error) {
	d.RLock()
	defer d.RUnlock()

	ce, exists := d.cacheFiles[cacheTag]
	if !exists || time.Now().After(ce.expires) {
		return 0, nil, ce.expires, fmt.Errorf("Not cached.")
	}

	data, err := ioutil.ReadFile(d.cacheRoot + "/" + cacheTag)
	if err != nil {
		delete(d.cacheFiles, cacheTag)
		return 0, nil, ce.expires, fmt.Errorf("Not cached.")
	}

	return ce.httpCode, data, ce.expires, nil
}

func (d *DiskCache) LogStats() {
	d.RLock()
	defer d.RUnlock()

	entries := 0
	expired := 0

	now := time.Now()
	for _, ce := range d.cacheFiles {
		entries++
		if now.After(ce.expires) {
			expired++
		}
	}

	log.Printf("Cache Entries: %d  Expired Entries: %d", entries, expired)
}

func NewDiskCache(rootDir string) *DiskCache {
	var dc DiskCache

	dc.cacheRoot = rootDir
	dc.cacheFiles = make(map[string]cacheEntry)

	dc.check()

	return &dc
}
