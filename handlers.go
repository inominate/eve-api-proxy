package main

import (
	"github.com/inominate/eve-api-proxy/apicache"
)

// Prototype for page specific handlers.
type APIHandler func(url string, params map[string]string) *apicache.Response

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
	"/char/locations.xml.aspx":              nil,
	"/char/mailbodies.xml.aspx":             nil,
	"/char/mailinglists.xml.aspx":           nil,
	"/char/mailmessages.xml.aspx":           nil,
	"/char/marketorders.xml.aspx":           nil,
	"/char/medals.xml.aspx":                 nil,
	"/char/notifications.xml.aspx":          nil,
	"/char/notificationtexts.xml.aspx":      nil,
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
	"/corp/locations.xml.aspx":              nil,
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
