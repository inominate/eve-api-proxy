// Copyright 2013

/*
EVE Online API downloader/cacher.


Begin by initializing the default client.
	NewClient(NilCache)

Create a new request for the API page needed.
	req := NewRequest("eve/ConquerableStationList.xml.aspx")

Set any options you may need, including keyid/vcode.
	req.Set("keyid", "1234")
	req.Set("vcode", "abcd")
	req.Set("charactername", "innominate")
	req.Set("characterid", fmt.Sprintf("%d", 123))

Get your XML.
	xml, err := req.Do()
*/
package apicache

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const sqlDateTime = "2006-01-02 15:04:05"

//
//  API Error Type, hopefully contains the error code and some useful information.
//  This is CCP so nothing is guaranteed.
//
type APIError struct {
	ErrorCode int    `xml:"code,attr"`
	ErrorText string `xml:",innerxml"`
}

// Error String Generator
func (e APIError) Error() string {
	return fmt.Sprintf("API Error %d: %s", e.ErrorCode, e.ErrorText)
}

// API Client structure.  Must be created using NewClient() function.
type Client struct {
	// Base URL, defaults to CCPs api but can be changed to a proxy
	BaseURL string

	// Default three retries, can be changed at will.
	Retries int

	timeout    time.Duration
	cacher     Cacher
	httpClient *http.Client

	panicUntil  time.Time
	panicCode   int
	panicReason string

	sync.RWMutex
}

// Default client, first client created lives here as well.
var client *Client

// Default BaseURL for new clients, change as necessary.  Must contain
// trailing slash or bad things happen.
var DefaultBaseURL = "https://api.eveonline.com/"

// Return the default client for
func GetDefaultClient() *Client {
	if client == nil {
		panic("Tried to get nonexistent client, must be initialized first.")
	}

	return client
}

// Create a new API Client.  The first time this is called will become
// the default client.  Requires a cacher.
func NewClient(cacher Cacher) *Client {
	var newClient Client

	newClient.BaseURL = DefaultBaseURL
	newClient.Retries = 3
	newClient.cacher = cacher

	// Also sets up our initial http client
	newClient.SetTimeout(60 * time.Second)

	if client == nil {
		client = &newClient
	}
	return &newClient
}

// Set timeout for each API request.
func (c *Client) SetTimeout(timeout time.Duration) {
	if timeout.Seconds() <= 0 || timeout.Seconds() > 3600 {
		timeout = 60 * time.Second
	}

	c.timeout = timeout
	c.httpClient = &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				deadline := time.Now().Add(timeout)
				c, err := net.DialTimeout(netw, addr, timeout)
				if err != nil {
					return nil, err
				}
				c.SetDeadline(deadline)
				return c, nil
			},
		},
	}
}

// Set timeout for default client.
func SetTimeout(timeout time.Duration) {
	client.SetTimeout(timeout)
}

// Create a new request.
func (c *Client) NewRequest(url string) *Request {
	var r Request
	r.params = make(map[string]string)

	if url[0] == '/' {
		url = url[1:]
	}
	r.url = url
	r.client = c

	return &r
}

// Create a new request using default client.
func NewRequest(url string) *Request {
	return client.NewRequest(url)
}

var (
	ErrCannotConnect = fmt.Errorf("Error connecting to API.")
	ErrNetwork       = fmt.Errorf("Network error.")
	ErrHTTP          = fmt.Errorf("HTTP error.")
	ErrForbidden     = fmt.Errorf("HTTP Forbidden, invalid API provided.")
	ErrUnknown       = fmt.Errorf("Unknown Error.")
	ErrXML           = fmt.Errorf("Malformed XML Detected")
	ErrTime          = fmt.Errorf("Malformed cache directive.")
)

type cacheResp struct {
	Error       APIError `xml:"error"`
	CachedUntil string   `xml:"cachedUntil"`
}

type Response struct {
	// Raw XML data
	Data []byte

	// Signals if we used the cache or not
	FromCache bool

	// Data expiration time
	Expires time.Time

	// true if an error occured due to invalid API rather than server problems
	Invalidate bool

	// Contains API Error if one occured
	Error APIError

	//Pass on CCP's HTTP code because why not?
	HTTPCode int
}

func (c *Client) GetCached(r *Request) (retresp Response, reterr error) {
	var resp Response

	// Check for cached version
	cacheTag := r.cacheTag()
	httpCode, data, expires, err := c.cacher.Get(cacheTag)
	if err == nil && !r.Force && !r.NoCache {
		resp.Data = data
		resp.FromCache = true
		resp.Expires = expires
		resp.HTTPCode = httpCode

		return resp, nil
	}
	return resp, fmt.Errorf("Not Cached")
}

// Perform a request, usually called by the request itself.
// User friendly error is enclosed in the response, returned error should be
// for internal use only.
func (c *Client) Do(r *Request) (retresp Response, reterr error) {
	var resp Response

	// Check for cached version
	cacheTag := r.cacheTag()
	httpCode, data, expires, err := c.cacher.Get(cacheTag)
	if err == nil && !r.Force && !r.NoCache {
		resp.Data = data
		resp.FromCache = true
		resp.Expires = expires
		resp.HTTPCode = httpCode

		return resp, nil
	}

	// If we're panicking, bail out early and spit back a fake error
	c.RLock()
	if c.panicUntil.After(time.Now()) {
		data := SynthesizeAPIError(c.panicCode, c.panicReason, c.panicUntil.Sub(time.Now()))
		c.RUnlock()

		resp.Data = data
		resp.FromCache = true
		resp.Expires = c.panicUntil
		resp.HTTPCode = 418
		resp.Error = APIError{c.panicCode, c.panicReason}

		return resp, nil
	}
	c.RUnlock()

	// Build parameter list
	formValues := make(url.Values)
	for k, v := range r.params {
		formValues.Set(k, v)
	}

	// Use defer to cache so we can synthesize error pages if necessary
	defer func() {
		if reterr != nil {
			resp.HTTPCode = 504
			resp.Data = SynthesizeAPIError(500, "APIProxy Error: "+reterr.Error(), 15*time.Minute)
		} else if resp.Data == nil {
			resp.HTTPCode = 504
			resp.Data = SynthesizeAPIError(900, "This shouldn't happen.", 15*time.Minute)
		}
		if !r.NoCache {
			err := c.cacher.Store(cacheTag, resp.HTTPCode, resp.Data, resp.Expires)
			if err != nil {
				log.Printf("Cache Error: %s", err)
			}
		}
	}()
	//Post the shit, retry if necessary.
	tries := 0
	var httpResp *http.Response
	for tries < c.Retries {
		tries++

		httpResp, err = c.httpClient.PostForm(c.BaseURL+r.url, formValues)
		if err != nil {
			log.Printf("Error Connecting to API: %s", err)
			time.Sleep(1 * time.Second)
			continue
		}
		defer httpResp.Body.Close()

		resp.HTTPCode = httpResp.StatusCode
		data, err = ioutil.ReadAll(httpResp.Body)
		if err != nil {
			log.Printf("Error Reading from API: %s", err)
			time.Sleep(1 * time.Second)
			continue
		}
	}
	if err != nil {
		log.Printf("Failed to access api: %s - %#v", c.BaseURL+r.url, formValues)
		return resp, ErrNetwork
	}

	//data = SynthesizeAPIError(904, "Your IP address has been temporarily blocked because it is causing too many errors. See the cacheUntil timestamp for when it will be opened again. IPs that continuall    y cause a lot of errors in the API will be permanently banned, please take measures to minimize problematic API calls from your application.", time.Second*30)

	// Get cache directive, bail with an error if anything is wrong with XML or
	// time format.  If these produce an error the rest of the data should be
	// considered worthless.
	var cR cacheResp
	err = xml.Unmarshal(data, &cR)
	if err != nil {
		log.Printf("XML Error: %s", err)
		return resp, ErrXML
	}

	// Get expiration
	expires, err = time.Parse(sqlDateTime, cR.CachedUntil)
	if err != nil {
		return resp, ErrTime
	}
	// Handle extended expiration requests.
	if r.Expires.After(expires) {
		resp.Expires = r.Expires
	} else {
		resp.Expires = expires
	}

	// Pass on any API errors
	resp.Error = cR.Error

	code := cR.Error.ErrorCode
	if code >= 901 && code <= 905 {
		log.Printf("Major API Error: %d - %s for %s %+v", cR.Error.ErrorCode, cR.Error.ErrorText, r.url, r.params)
		log.Printf("Pausing all API actions until %s...", resp.Expires.Format(sqlDateTime))
		c.Lock()
		c.panicUntil = resp.Expires
		c.panicCode = code
		c.panicReason = cR.Error.ErrorText
		c.Unlock()
	}
	if resp.HTTPCode == 403 || (code >= 100 && code <= 299) {
		resp.Invalidate = true
	}

	resp.Data = data
	return resp, nil
}
