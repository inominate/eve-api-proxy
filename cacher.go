package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

var prefixes = "0123456789abcdef"

type cacheEntry struct {
	httpCode int
	expires  time.Time
}

type DiskCache struct {
	cacheRoot  string
	cacheFiles map[string]cacheEntry
	sync.RWMutex
}

func (d *DiskCache) init() {
	d.Lock()
	defer d.Unlock()

	os.Mkdir(d.cacheRoot, 0770)
	for _, dir := range prefixes {
		os.RemoveAll(d.cacheRoot + "/" + string(dir))
		os.Mkdir(d.cacheRoot+"/"+string(dir), 0770)
	}
}

var cleanOnce = &sync.Once{}

func (d *DiskCache) clean() {
	log.Printf("Cleaning Up.")
	now := time.Now()

	cleancount := 0
	for tag, ce := range d.cacheFiles {
		if now.After(ce.expires) {
			os.Remove(d.cacheRoot + "/" + string(tag[0]) + "/" + tag)
			delete(d.cacheFiles, tag)

			cleancount++
		}
	}
	log.Printf("Cleaned up %d entries.", cleancount)
	cleanOnce = &sync.Once{}
}

var storeCount = 0

func (d *DiskCache) Store(cacheTag string, httpCode int, data []byte, expires time.Time) error {
	d.Lock()
	defer d.Unlock()

	storeCount++
	if storeCount >= 500 {
		storeCount = 0
		go cleanOnce.Do(func() { d.clean() })
	}

	ce := cacheEntry{httpCode, expires}
	err := ioutil.WriteFile(d.cacheRoot+"/"+string(cacheTag[0])+"/"+cacheTag, data, 0660)
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

	data, err := ioutil.ReadFile(d.cacheRoot + "/" + string(cacheTag[0]) + "/" + cacheTag)
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

	dc.init()

	return &dc
}
