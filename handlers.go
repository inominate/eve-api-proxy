package main

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/inominate/eve-api-proxy/apicache"
)

// Prototype for page specific handlers.
type APIHandler func(url string, params map[string]string) *apicache.Response

// A default API handler, does a straight pull with no mangling.
func defaultHandler(url string, params map[string]string) *apicache.Response {
	resp, err := APIReq(url, params)

	if err != nil {
		debugLog.Printf("API Error %s: %s - %+v", err, url, params)
	}
	return resp
}

// Defines valid API pages and what special handler they should use.
// nil handlers will attempt to use defaultHandler which is a straight
// passthrough.
var validPages = map[string]APIHandler{
	//	"/control/":                             controlHandler,
	"/account/accountstatus.xml.aspx":       nil,
	"/account/apikeyinfo.xml.aspx":          apiKeyInfoHandler,
	"/account/characters.xml.aspx":          nil,
	"/char/accountbalance.xml.aspx":         nil,
	"/char/assetlist.xml.aspx":              nil,
	"/char/calendareventattendees.xml.aspx": nil,
	"/char/charactersheet.xml.aspx":         nil,
	"/char/contactlist.xml.aspx":            nil,
	"/char/contactnotifications.xml.aspx":   nil,
	"/char/contracts.xml.aspx":              nil,
	"/char/contractitems.xml.aspx":          nil,
	"/char/contractbids.xml.aspx":           nil,
	"/char/facwarstats.xml.aspx":            nil,
	"/char/industryjobs.xml.aspx":           nil,
	"/char/killlog.xml.aspx":                nil,
	"/char/locations.xml.aspx":              idsListHandler,
	"/char/mailbodies.xml.aspx":             idsListHandler,
	"/char/mailinglists.xml.aspx":           nil,
	"/char/mailmessages.xml.aspx":           nil,
	"/char/marketorders.xml.aspx":           nil,
	"/char/medals.xml.aspx":                 nil,
	"/char/notifications.xml.aspx":          nil,
	"/char/notificationtexts.xml.aspx":      idsListHandler,
	"/char/research.xml.aspx":               nil,
	"/char/skillintraining.xml.aspx":        nil,
	"/char/skillqueue.xml.aspx":             nil,
	"/char/standings.xml.aspx":              nil,
	"/char/upcomingcalendarevents.xml.aspx": nil,
	"/char/walletjournal.xml.aspx":          nil,
	"/char/wallettransactions.xml.aspx":     nil,
	"/corp/accountbalance.xml.aspx":         nil,
	"/corp/assetlist.xml.aspx":              nil,
	"/corp/contactlist.xml.aspx":            nil,
	"/corp/containerlog.xml.aspx":           nil,
	"/corp/contracts.xml.aspx":              nil,
	"/corp/contractitems.xml.aspx":          nil,
	"/corp/contractbids.xml.aspx":           nil,
	"/corp/corporationsheet.xml.aspx":       nil,
	"/corp/facwarstats.xml.aspx":            nil,
	"/corp/industryjobs.xml.aspx":           nil,
	"/corp/killlog.xml.aspx":                nil,
	"/corp/locations.xml.aspx":              idsListHandler,
	"/corp/marketorders.xml.aspx":           nil,
	"/corp/medals.xml.aspx":                 nil,
	"/corp/membermedals.xml.aspx":           nil,
	"/corp/membersecurity.xml.aspx":         nil,
	"/corp/membersecuritylog.xml.aspx":      nil,
	"/corp/membertracking.xml.aspx":         nil,
	"/corp/outpostlist.xml.aspx":            nil,
	"/corp/outpostservicedetail.xml.aspx":   nil,
	"/corp/shareholders.xml.aspx":           nil,
	"/corp/standings.xml.aspx":              nil,
	"/corp/starbasedetail.xml.aspx":         nil,
	"/corp/starbaselist.xml.aspx":           nil,
	"/corp/titles.xml.aspx":                 nil,
	"/corp/walletjournal.xml.aspx":          nil,
	"/corp/wallettransactions.xml.aspx":     nil,
	"/eve/alliancelist.xml.aspx":            nil,
	"/eve/certificatetree.xml.aspx":         nil,
	"/eve/characterid.xml.aspx":             nil,
	"/eve/characterinfo.xml.aspx":           nil,
	"/eve/charactername.xml.aspx":           nil,
	"/eve/conquerablestationlist.xml.aspx":  nil,
	"/eve/errorlist.xml.aspx":               nil,
	"/eve/facwarstats.xml.aspx":             nil,
	"/eve/facwartopstats.xml.aspx":          nil,
	"/eve/reftypes.xml.aspx":                nil,
	"/eve/skilltree.xml.aspx":               nil,
	"/eve/typename.xml.aspx":                nil,
	"/map/facwarsystems.xml.aspx":           nil,
	"/map/jumps.xml.aspx":                   nil,
	"/map/kills.xml.aspx":                   nil,
	"/map/sovereignty.xml.aspx":             nil,
	"/map/sovereigntystatus.xml.aspx":       nil,
	"/server/serverstatus.xml.aspx":         nil,
	"/api/calllist.xml.aspx":                nil,
}

// Bug Correcting Handler for APIKeyInfo.xml.aspx
// API occasionally returns 221s for no reason, retry automatically when we
// run into one of them.
func apiKeyInfoHandler(url string, params map[string]string) *apicache.Response {
	resp, err := APIReq(url, params)

	// :ccp: 221's come up for no reason and need to be ignored
	if err == nil && resp.Error.ErrorCode == 221 {
		params["force"] = "true"

		for i := 0; i < conf.Retries; i++ {
			resp, err = APIReq(url, params)
			if resp.Error.ErrorCode != 221 || err != nil {
				break
			}
		}
	}

	if err != nil {
		debugLog.Printf("API Error %s: %s - %+v", err, url, params)
	}
	return resp
}

const maxIDErrors = 16

// Bug Correcting Handler for endpoints using comma separated ID lists which
// will fail entirely in case of a single invalid ID.
//
// Note: Can generate many errors so should only be used with applications
// that know to behave themselves. Add a form value of fix with any content
// to enable the correction.
func idsListHandler(url string, params map[string]string) *apicache.Response {
	var runFixer bool
	if _, ok := params["fix"]; ok {
		delete(params, "fix")
		runFixer = true
	}

	resp, err := APIReq(url, params)
	if err != nil {
		debugLog.Printf("API Error %s: %s - %+v", err, url, params)
	}
	if !runFixer {
		return resp
	}

	var ids []string
	if idsParam, ok := params["ids"]; ok {
		ids = strings.Split(idsParam, ",")
	}

	// If we have no ids or just one, we're not doing anything special.
	// If there's more than 250 ids, that's beyond the API limit so we won't
	// touch that either.
	if len(ids) == 0 || len(ids) == 1 || len(ids) > 250 {
		return resp
	}
	// If the request didn't have an invalid id, errorcode 135, there's nothing
	// we can do to help.
	if resp.Error.ErrorCode != 135 {
		return resp
	}

	// If we got this far there's more than one ID, at least one of which is
	// invalid.
	debugLog.Printf("idsListHandler going into action for %d ids: %s", len(ids), params["ids"])

	var errCount errCount
	delete(params, "ids")

	validIDs, err := findValidIDs(url, params, ids, &errCount)
	if err != nil {
		debugLog.Printf("findValidIDs failed: %s", err)
		return resp
	}

	idsBuf := &bytes.Buffer{}
	fmt.Fprintf(idsBuf, "%s", validIDs[0])
	for i := 1; i < len(validIDs); i++ {
		fmt.Fprintf(idsBuf, ",%s", validIDs[i])
	}
	idsParam := idsBuf.String()
	params["ids"] = idsParam

	resp, err = APIReq(url, params)
	if err != nil {
		debugLog.Printf("API Error %s: %s - %+v", err, url, params)
	}
	debugLog.Printf("Completed with: %d errors.", errCount.Get())
	return resp
}

type errCount struct {
	count int
	sync.Mutex
}

func (e *errCount) Get() int {
	return e.count
}
func (e *errCount) Add() int {
	e.count++
	return e.count
}

func findValidIDs(url string, params map[string]string, ids []string, errCount *errCount) ([]string, error) {
	if false && len(ids) == 1 {
		valid, err := isValidIDList(url, params, ids, errCount)
		if valid {
			return ids, err
		} else {
			return nil, err
		}
	}

	var leftIDs, rightIDs []string
	var leftErr, rightErr error

	left := ids[0 : len(ids)/2]
	leftValid, leftErr := isValidIDList(url, params, left, errCount)
	if leftErr != nil {
		return nil, leftErr
	}
	if leftValid {
		leftIDs = left
	} else {
		if len(left) > 1 {
			leftIDs, leftErr = findValidIDs(url, params, left, errCount)
			if rightErr != nil {
				return nil, leftErr
			}
		}
	}

	right := ids[len(ids)/2:]
	rightValid, rightErr := isValidIDList(url, params, right, errCount)
	if rightErr != nil {
		return nil, rightErr
	}
	if rightValid {
		rightIDs = right
	} else {
		if len(right) > 1 {
			rightIDs, rightErr = findValidIDs(url, params, right, errCount)
			if rightErr != nil {
				return nil, rightErr
			}
		}
	}

	validIDs := append(leftIDs, rightIDs...)
	return validIDs, nil
}

func isValidIDList(url string, params map[string]string, ids []string, errCount *errCount) (bool, error) {
	errCount.Lock()
	defer errCount.Unlock()

	if count := errCount.Get(); count >= maxIDErrors {
		return false, fmt.Errorf("failed to get ids, hit %d errors limit", count)
	}

	idsBuf := &bytes.Buffer{}
	fmt.Fprintf(idsBuf, "%s", ids[0])
	for i := 1; i < len(ids); i++ {
		fmt.Fprintf(idsBuf, ",%s", ids[i])
	}
	idsParam := idsBuf.String()

	var newParams = make(map[string]string)
	for k, v := range params {
		newParams[k] = v
	}
	newParams["ids"] = idsParam

	resp, err := APIReq(url, newParams)
	// Bail completely if the API itself fails for any reason.
	if err != nil {
		return false, err
	}
	// If there is no error then this batch is okay.
	if resp.Error.ErrorCode == 0 {
		return true, nil
	}
	// Bail if we got a non-api failure error other than invalid ID
	if resp.Error.ErrorCode != 135 {
		return false, resp.Error
	}

	debugLog.Printf("Adding Error %d for: %v", errCount.Get(), ids)
	errCount.Add()

	return false, nil
}
