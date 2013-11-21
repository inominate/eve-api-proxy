package apicache

import (
	"crypto/sha1"
	"fmt"
	"sort"
	"strings"
	"time"
)

// API Request structure for making requests.
type Request struct {
	// Contains the final url only "section/api.xml.asp..."
	url string

	params map[string]string
	client *Client

	// Override the CCP requested expiration time, will not reduce existing cache
	// duration, see Force below. Zero value indicates this field should be
	// ignored
	// Intended use is to force longer cache timers to align dependent calls.
	// e.g. Pull starbase and locations APIs in sync with assets.
	Expires time.Time

	// Force pull despite cache, use at own risk.
	Force bool

	// Do not cache this request, use at own risk.
	NoCache bool
}

// Set parameters for the API call.
// KeyID/vCode can be set here and will override the client's settings.
func (r *Request) Set(key, value string) {
	key = strings.ToLower(strings.TrimSpace(key))

	r.params[key] = value
}

// Perform a request.
// User friendly error is enclosed in the response, returned error should be
// for internal use only.
func (r *Request) Do() (*Response, error) {
	return r.client.Do(r)
}

// Get the cached request, or return an error if not cached.
// User friendly error is enclosed in the response, returned error should be
// for internal use only.
func (r *Request) GetCached() (*Response, error) {
	return r.client.GetCached(r)
}

// Generate a unique cache tag for this request.
func (r *Request) cacheTag() string {
	var keys []string
	for k := range r.params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	s := sha1.New()
	for _, key := range keys {
		fmt.Fprintf(s, "%s: %s\n", key, r.params[key])
	}
	fmt.Fprintf(s, "%s", r.url)
	return fmt.Sprintf("%x", s.Sum(nil))
}
