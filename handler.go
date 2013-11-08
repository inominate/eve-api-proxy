package main

import (
	"ieveapi/apicache"
	"log"
	"net/http"
	"path"
	"runtime"
	"time"
)

type APIHandler struct{}

func (a APIHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	url := path.Clean(req.URL.Path)

	if url == "/stats" {
		LogStats()
		w.Write([]byte(""))
		return
	}

	w.Header().Add("Content-Type", "text/xml")
	if _, valid := validPages[url]; !valid {
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
		log.Printf("Handler Error: %s", err)
	}

	w.WriteHeader(code)
	w.Write(data)
}

func LogStats() {
	PrintWorkerStats()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("Alloc: %dkb Sys: %dkb", m.Alloc/1024, m.Sys/1024)
	log.Printf("HeapAlloc: %dkb HeapSys: %dkb", m.HeapAlloc/1024, m.HeapSys/1024)
}

var validPages = map[string]bool{
	"/account/AccountStatus.xml.aspx":       true,
	"/account/APIKeyInfo.xml.aspx":          true,
	"/account/Characters.xml.aspx":          true,
	"/char/AccountBalance.xml.aspx":         true,
	"/char/AssetList.xml.aspx":              true,
	"/char/CalendarEventAttendees.xml.aspx": true,
	"/char/CharacterSheet.xml.aspx":         true,
	"/char/ContactList.xml.aspx":            true,
	"/char/ContactNotifications.xml.aspx":   true,
	"/char/Contracts.xml.aspx":              true,
	"/char/ContractItems.xml.aspx":          true,
	"/char/ContractBids.xml.aspx":           true,
	"/char/FacWarStats.xml.aspx":            true,
	"/char/IndustryJobs.xml.aspx":           true,
	"/char/Killlog.xml.aspx":                true,
	"/char/Locations.xml.aspx":              true,
	"/char/MailBodies.xml.aspx":             true,
	"/char/MailingLists.xml.aspx":           true,
	"/char/MailMessages.xml.aspx":           true,
	"/char/MarketOrders.xml.aspx":           true,
	"/char/Medals.xml.aspx":                 true,
	"/char/Notifications.xml.aspx":          true,
	"/char/NotificationTexts.xml.aspx":      true,
	"/char/Research.xml.aspx":               true,
	"/char/SkillInTraining.xml.aspx":        true,
	"/char/SkillQueue.xml.aspx":             true,
	"/char/Standings.xml.aspx":              true,
	"/char/UpcomingCalendarEvents.xml.aspx": true,
	"/char/WalletJournal.xml.aspx":          true,
	"/char/WalletTransactions.xml.aspx":     true,
	"/corp/AccountBalance.xml.aspx":         true,
	"/corp/AssetList.xml.aspx":              true,
	"/corp/ContactList.xml.aspx":            true,
	"/corp/ContainerLog.xml.aspx":           true,
	"/corp/Contracts.xml.aspx":              true,
	"/corp/ContractItems.xml.aspx":          true,
	"/corp/ContractBids.xml.aspx":           true,
	"/corp/CorporationSheet.xml.aspx":       true,
	"/corp/FacWarStats.xml.aspx":            true,
	"/corp/IndustryJobs.xml.aspx":           true,
	"/corp/Killlog.xml.aspx":                true,
	"/corp/Locations.xml.aspx":              true,
	"/corp/MarketOrders.xml.aspx":           true,
	"/corp/Medals.xml.aspx":                 true,
	"/corp/MemberMedals.xml.aspx":           true,
	"/corp/MemberSecurity.xml.aspx":         true,
	"/corp/MemberSecurityLog.xml.aspx":      true,
	"/corp/MemberTracking.xml.aspx":         true,
	"/corp/OutpostList.xml.aspx":            true,
	"/corp/OutpostServiceDetail.xml.aspx":   true,
	"/corp/Shareholders.xml.aspx":           true,
	"/corp/Standings.xml.aspx":              true,
	"/corp/StarbaseDetail.xml.aspx":         true,
	"/corp/StarbaseList.xml.aspx":           true,
	"/corp/Titles.xml.aspx":                 true,
	"/corp/WalletJournal.xml.aspx":          true,
	"/corp/WalletTransactions.xml.aspx":     true,
	"/eve/AllianceList.xml.aspx":            true,
	"/eve/CertificateTree.xml.aspx":         true,
	"/eve/CharacterID.xml.aspx":             true,
	"/eve/CharacterInfo.xml.aspx":           true,
	"/eve/CharacterName.xml.aspx":           true,
	"/eve/ConquerableStationList.xml.aspx":  true,
	"/eve/ErrorList.xml.aspx":               true,
	"/eve/FacWarStats.xml.aspx":             true,
	"/eve/FacWarTopStats.xml.aspx":          true,
	"/eve/RefTypes.xml.aspx":                true,
	"/eve/SkillTree.xml.aspx":               true,
	"/eve/TypeName.xml.aspx":                true,
	"/map/FacWarSystems.xml.aspx":           true,
	"/map/Jumps.xml.aspx":                   true,
	"/map/Kills.xml.aspx":                   true,
	"/map/Sovereignty.xml.aspx":             true,
	"/map/SovereigntyStatus.xml.aspx":       true,
	"/server/ServerStatus.xml.aspx":         true,
	"/api/calllist.xml.aspx":                true,
}
