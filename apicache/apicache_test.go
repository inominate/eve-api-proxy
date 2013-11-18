package apicache

import (
	"database/sql"
	"flag"
	_ "github.com/go-sql-driver/mysql"
	"testing"
	"time"
)

var DSN = flag.String("dsn", "", "Database DSN for API Cache Testing.")

func Test_API(t *testing.T) {
	NewClient(NilCache)

	req := NewRequest("eve/ConquerableStationList.xml.aspx")
	resp, err := req.Do()

	if err != nil {
		t.Errorf("req.Do() error: %s", err)
		return
	}

	if len(resp.Data) == 0 {
		t.Error("No data returned")
		return
	}

	if resp.Expires.Before(time.Now()) {
		t.Error("Expiration time invalid")
		return
	}

	t.Log("Success.")
}

func Test_API_2(t *testing.T) {
	if *DSN == "" {
		t.Log("SQL cacher untested. Please re-run with -dsn=\"go-mysql-driver dsn\"")
		return
	}

	db, err := sql.Open("mysql", *DSN)
	if err != nil {
		t.Errorf("Error opening database: %s", err)
		return
	}

	sqlCacher, err := SQLCacher(db)
	if err != nil {
		t.Errorf("Error initializing SQL Cacher: %s", err)
		return
	}

	cl := NewClient(sqlCacher)

	req := cl.NewRequest("eve/ConquerableStationList.xml.aspx")
	resp, err := req.Do()

	if len(resp.Data) == 0 {
		t.Error("No data returned")
		return
	}

	if resp.Expires.Before(time.Now()) {
		t.Error("Expiration time invalid")
		return
	}

	req = cl.NewRequest("eve/ConquerableStationList.xml.aspx")
	resp, err = req.Do()

	if req.cacheTag() != "8c9e9d9868b287a027082b275880b2f2d0cee785" {
		t.Error("Invalid Cache Tag")
		return
	}

	if !resp.FromCache {
		t.Error("Cache not functional.")
		return
	}

	req = cl.NewRequest("eve/ConquerableStationList.xml.aspx")
	req.Force = true
	resp, err = req.Do()

	if resp.FromCache {
		t.Error("Got cached data, didn't want it")
		return
	}

	t.Log("Success.")
}
