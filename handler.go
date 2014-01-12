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

type APIHandler struct{}

func (a APIHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
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
	if _, valid := validPages[strings.ToLower(url)]; !valid {
		log.Printf("Invalid URL %s - %s", url, req.Form)
		w.WriteHeader(404)
		w.Write(apicache.SynthesizeAPIError(404, "Invalid API page.", 24*time.Hour))
		return
	}

	params := make(map[string]string)
	for key, val := range req.Form {
		params[key] = val[0]
	}

	data, code, err := APIReq(url, params)
	if err != nil {
		log.Printf("Handler Error for %s: %s - %+v", err, url, params)
	}

	w.WriteHeader(code)
	w.Write(data)

	if useLog >= 4 {
		log.Printf("Request took: %.2f seconds.", time.Since(startTime).Seconds())
	} else if useLog >= 2 && time.Since(startTime).Seconds() > 10 {
		log.Printf("Slow Request took %.2f seconds:", time.Since(startTime).Seconds())
		log.Printf("Request for %s: %+v", url, params)
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

var validPages = map[string]bool{
	"/account/accountstatus.xml.aspx":       true,
	"/account/apikeyinfo.xml.aspx":          true,
	"/account/characters.xml.aspx":          true,
	"/char/accountbalance.xml.aspx":         true,
	"/char/assetlist.xml.aspx":              true,
	"/char/calendareventattendees.xml.aspx": true,
	"/char/charactersheet.xml.aspx":         true,
	"/char/contactlist.xml.aspx":            true,
	"/char/contactnotifications.xml.aspx":   true,
	"/char/contracts.xml.aspx":              true,
	"/char/contractitems.xml.aspx":          true,
	"/char/contractbids.xml.aspx":           true,
	"/char/facwarstats.xml.aspx":            true,
	"/char/industryjobs.xml.aspx":           true,
	"/char/killlog.xml.aspx":                true,
	"/char/locations.xml.aspx":              true,
	"/char/mailbodies.xml.aspx":             true,
	"/char/mailinglists.xml.aspx":           true,
	"/char/mailmessages.xml.aspx":           true,
	"/char/marketorders.xml.aspx":           true,
	"/char/medals.xml.aspx":                 true,
	"/char/notifications.xml.aspx":          true,
	"/char/notificationtexts.xml.aspx":      true,
	"/char/research.xml.aspx":               true,
	"/char/skillintraining.xml.aspx":        true,
	"/char/skillqueue.xml.aspx":             true,
	"/char/standings.xml.aspx":              true,
	"/char/upcomingcalendarevents.xml.aspx": true,
	"/char/walletjournal.xml.aspx":          true,
	"/char/wallettransactions.xml.aspx":     true,
	"/corp/accountbalance.xml.aspx":         true,
	"/corp/assetlist.xml.aspx":              true,
	"/corp/contactlist.xml.aspx":            true,
	"/corp/containerlog.xml.aspx":           true,
	"/corp/contracts.xml.aspx":              true,
	"/corp/contractitems.xml.aspx":          true,
	"/corp/contractbids.xml.aspx":           true,
	"/corp/corporationsheet.xml.aspx":       true,
	"/corp/facwarstats.xml.aspx":            true,
	"/corp/industryjobs.xml.aspx":           true,
	"/corp/killlog.xml.aspx":                true,
	"/corp/locations.xml.aspx":              true,
	"/corp/marketorders.xml.aspx":           true,
	"/corp/medals.xml.aspx":                 true,
	"/corp/membermedals.xml.aspx":           true,
	"/corp/membersecurity.xml.aspx":         true,
	"/corp/membersecuritylog.xml.aspx":      true,
	"/corp/membertracking.xml.aspx":         true,
	"/corp/outpostlist.xml.aspx":            true,
	"/corp/outpostservicedetail.xml.aspx":   true,
	"/corp/shareholders.xml.aspx":           true,
	"/corp/standings.xml.aspx":              true,
	"/corp/starbasedetail.xml.aspx":         true,
	"/corp/starbaselist.xml.aspx":           true,
	"/corp/titles.xml.aspx":                 true,
	"/corp/walletjournal.xml.aspx":          true,
	"/corp/wallettransactions.xml.aspx":     true,
	"/eve/alliancelist.xml.aspx":            true,
	"/eve/certificatetree.xml.aspx":         true,
	"/eve/characterid.xml.aspx":             true,
	"/eve/characterinfo.xml.aspx":           true,
	"/eve/charactername.xml.aspx":           true,
	"/eve/conquerablestationlist.xml.aspx":  true,
	"/eve/errorlist.xml.aspx":               true,
	"/eve/facwarstats.xml.aspx":             true,
	"/eve/facwartopstats.xml.aspx":          true,
	"/eve/reftypes.xml.aspx":                true,
	"/eve/skilltree.xml.aspx":               true,
	"/eve/typename.xml.aspx":                true,
	"/map/facwarsystems.xml.aspx":           true,
	"/map/jumps.xml.aspx":                   true,
	"/map/kills.xml.aspx":                   true,
	"/map/sovereignty.xml.aspx":             true,
	"/map/sovereigntystatus.xml.aspx":       true,
	"/server/serverstatus.xml.aspx":         true,
	"/api/calllist.xml.aspx":                true,
}
