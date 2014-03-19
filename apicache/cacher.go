package apicache

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Cachers MUST be safe for concurrent use.
type Cacher interface {
	Store(cacheTag string, httpCode int, data []byte, expires time.Time) error
	Get(cacheTag string) (int, []byte, time.Time, error)
}

type sqlCache struct {
	db *sql.DB

	getStmt     *sql.Stmt
	storeStmt   *sql.Stmt
	cleanUpStmt *sql.Stmt
}

// SQL Database Cacher
// Must be passed an existing database handle, returns a cacher which can be
// used with NewClient().  Will create its own table if necessary.
func SQLCacher(db *sql.DB) (*sqlCache, error) {
	_, err := db.Query(`
		CREATE TABLE IF NOT EXISTS apicache (
			cacheid char(40) NOT NULL,
			httpCode integer NOT NULL,
			data longtext NOT NULL,
			created timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
			PRIMARY KEY (cacheid),
			KEY expires (expires)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8`)
	if err != nil {
		return nil, err
	}

	var c sqlCache
	c.db = db

	c.getStmt, err = db.Prepare("select data, httpCode, expires from apicache where cacheid = ? and expires > utc_timestamp()")
	if err != nil {
		log.Printf("Error Preparing SQL cache get: %s", err)
		return nil, err
	}

	c.storeStmt, err = db.Prepare("replace into apicache (`cacheid`, `httpCode`, `data`, `expires`) VALUES (?, ?, ?, ?)")
	if err != nil {
		log.Printf("Error Preparing SQL cache store: %s", err)
		return nil, err
	}

	c.cleanUpStmt, err = db.Prepare("delete from apicache where expires < utc_timestamp()")
	if err != nil {
		log.Printf("Error Preparing SQL cache cleanup: %s", err)
		return nil, err
	}

	go c.cleanUp()
	return &c, nil
}

// Store entry in cache.
func (c *sqlCache) Store(cacheTag string, httpCode int, data []byte, expires time.Time) error {
	_, err := c.storeStmt.Exec(cacheTag, httpCode, data, expires)
	if err != nil {
		return err
	}
	return nil
}

// Retrieve entry from cache, return error if no unexpired entry is available.  Returns data, expiration time, and error.
func (c *sqlCache) Get(cacheTag string) (int, []byte, time.Time, error) {
	var httpCode int
	var data []byte
	var expires time.Time

	err := c.getStmt.QueryRow(cacheTag).Scan(&data, &httpCode, &expires)

	return httpCode, data, expires, err
}

// Background routine to regularly remove old data
func (c *sqlCache) cleanUp() {
	for {
		_, err := c.cleanUpStmt.Exec()
		if err != nil {
			log.Printf("cleanUp SQL Error: %s", err)
		}
		time.Sleep(1 * time.Hour)
	}
}

type nilCache struct{}

// Fake Cacher, does nothing but can serve as a stand-in for testing.
var NilCache = new(nilCache)

func (c *nilCache) Store(cacheTag string, httpCode int, data []byte, expires time.Time) error {
	DebugLog.Printf("Pretending to store %s  code: %d expires %s", cacheTag, httpCode, expires)
	return nil
}
func (c *nilCache) Get(cacheTag string) (int, []byte, time.Time, error) {
	var t time.Time
	return 0, nil, t, fmt.Errorf("No Error, NilCacher")
}
