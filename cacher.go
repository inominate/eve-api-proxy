package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

var prefixes = "0123456789abcdef"

type CacheEntry struct {
	HTTPCode int
	Expires  time.Time
}

type DiskCache struct {
	cacheRoot  string
	cacheFiles map[string]CacheEntry
	sync.RWMutex
}

func (d *DiskCache) init() {
	d.Lock()
	defer d.Unlock()

	if d.cacheFiles == nil {
		log.Fatalf("Tried to load uninitialized cache.")
	}

	os.Mkdir(d.cacheRoot, 0770)

	for _, dir := range prefixes {
		dirName := d.cacheRoot + "/" + string(dir)
		err := os.Mkdir(dirName, 0770)
		if err != nil {
			//Either cannot create directory, or directory already exists, let's try opening it to find out.
			dirf, derr := os.Open(dirName)
			if derr != nil {
				// Couldn't open directory, panic.
				log.Fatalf("Couldn't create or open %s: %s/%s", dirName, err, derr)
			}

			files, err := dirf.Readdirnames(0)
			if err != nil {
				log.Fatalf("Couldn't read %s: %s", dirName, err)
			}

			var de CacheEntry
			for _, filename := range files {
				if len(filename) < 5 || filename[len(filename)-len(".xml"):] == ".xml" {
					continue
				}
				fullname := dirName + "/" + filename

				jsondata, err := ioutil.ReadFile(fullname)
				if err != nil {
					log.Fatalf("Failed to read %s: %s", fullname, err)
				}

				err = json.Unmarshal(jsondata, &de)
				if err != nil {
					log.Printf("Recovering from cache consistency error for %s: %s ", fullname, err)
				}

				if err != nil || time.Now().After(de.Expires) {
					err := os.Remove(fullname)
					errx := os.Remove(fullname + ".xml")
					if err != nil || errx != nil {
						log.Fatalf("Failed to remove expired cache entry %s: %s - %s", fullname, err, errx)
					}
					continue
				}

				d.cacheFiles[filename] = de
			}
		}
	}
}

func (d *DiskCache) clean() {
	d.Lock()
	defer d.Unlock()

	if d.cacheFiles == nil {
		log.Fatalf("Tried to clean with uninitialized cache.")
	}

	log.Printf("Clearing existing cache.")

	os.Mkdir(d.cacheRoot, 0770)

	for _, dir := range prefixes {
		dirName := d.cacheRoot + "/" + string(dir)
		os.RemoveAll(dirName)
		os.Mkdir(dirName, 0770)
	}
}

func (d *DiskCache) expiredPurger() {
	for {
		debugLog.Printf("Cleaning Up.")
		now := time.Now()

		d.Lock()
		collectcount := 0
		for tag, ce := range d.cacheFiles {
			if now.After(ce.Expires) {
				os.Remove(d.filename(tag))
				os.Remove(d.filename(tag) + ".xml")
				delete(d.cacheFiles, tag)

				collectcount++
			}
		}
		d.Unlock()
		debugLog.Printf("Collected %d expired entries.", collectcount)

		time.Sleep(30 * time.Minute)
	}
}

var storeCount int64

func (d *DiskCache) filename(tag string) string {
	return d.cacheRoot + "/" + string(tag[0]) + "/" + tag
}

func (d *DiskCache) Store(cacheTag string, HTTPCode int, data []byte, Expires time.Time) error {
	d.Lock()
	defer d.Unlock()

	if d.cacheFiles == nil {
		log.Fatalf("Tried to store to uninitialized cache.")
	}

	ce := CacheEntry{HTTPCode, Expires}

	jsondata, err := json.Marshal(&ce)
	if err != nil {
		log.Printf("Unknown JSON Marshal Error: %s", err)
		return err
	}

	err = ioutil.WriteFile(d.filename(cacheTag), jsondata, 0660)
	if err != nil {
		log.Printf("Unknown File Error: %s", err)
		return err
	}

	err = ioutil.WriteFile(d.filename(cacheTag)+".xml", data, 0660)
	if err != nil {
		log.Printf("Unknown File Error: %s", err)
		return err
	}

	d.cacheFiles[cacheTag] = ce
	return nil
}

func (d *DiskCache) Get(cacheTag string) (int, []byte, time.Time, error) {
	d.RLock()
	defer d.RUnlock()

	if d.cacheFiles == nil {
		log.Fatalf("Tried to get from uninitialized cache.")
	}

	ce, exists := d.cacheFiles[cacheTag]
	if !exists || time.Now().After(ce.Expires) {
		return 0, nil, ce.Expires, fmt.Errorf("Not cached.")
	}

	jsondata, err := ioutil.ReadFile(d.filename(cacheTag))
	if err != nil {
		d.RUnlock()
		d.Lock()
		delete(d.cacheFiles, cacheTag)
		d.Unlock()
		d.RLock()

		return 0, nil, ce.Expires, fmt.Errorf("Cache error - metadata file not found.")
	}

	var de CacheEntry
	err = json.Unmarshal(jsondata, &de)
	if err != nil || de.Expires != ce.Expires {
		log.Printf("Cache consistency error: %s (Got: %s Expected: %s)", err, de.Expires, ce.Expires)

		d.RUnlock()
		d.Lock()
		delete(d.cacheFiles, cacheTag)
		d.Unlock()

		return 0, nil, ce.Expires, fmt.Errorf("Cache error - cache invalid.")
	}

	data, err := ioutil.ReadFile(d.filename(cacheTag) + ".xml")
	if err != nil {
		d.RUnlock()
		d.Lock()
		delete(d.cacheFiles, cacheTag)
		d.Unlock()

		return 0, nil, ce.Expires, fmt.Errorf("Cache error - data file not found.")
	}

	return ce.HTTPCode, data, ce.Expires, nil
}

func (d *DiskCache) LogStats(w io.Writer) {
	d.RLock()
	defer d.RUnlock()

	entries := 0
	expired := 0

	now := time.Now()
	for _, ce := range d.cacheFiles {
		entries++
		if now.After(ce.Expires) {
			expired++
		}
	}

	fmt.Fprintf(w, "Cache Entries: %d  Expired Entries: %d\n", entries, expired)
}

func NewDiskCache(rootDir string, clearCache bool) *DiskCache {
	var dc DiskCache

	dc.cacheRoot = rootDir
	dc.cacheFiles = make(map[string]CacheEntry)

	if clearCache {
		dc.clean()
	} else {
		dc.init()
	}

	go dc.expiredPurger()
	return &dc
}
