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

Get your response.
	resp, err := req.Do()
	xml := resp.Data
*/
package apicache

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const sqlDateTime = "2006-01-02 15:04:05"

var DebugLog = log.New(ioutil.Discard, "apicache", log.Ldate|log.Ltime)

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

	timeout      time.Duration
	maxIdleConns int

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
	newClient.Retries = 5
	newClient.cacher = cacher
	newClient.maxIdleConns = 2

	// Also sets up our initial http client
	newClient.SetTimeout(60 * time.Second)

	if client == nil {
		client = &newClient
	}
	return &newClient
}

func (c *Client) SetMaxIdleConns(maxIdleConns int) {
	// Enforce some sanity.
	if maxIdleConns <= 0 || maxIdleConns >= 64 {
		maxIdleConns = 2
	}
	c.maxIdleConns = maxIdleConns
	c.newHttpClient()
}

// Set timeout for each API request.
func (c *Client) SetTimeout(timeout time.Duration) {
	if timeout.Seconds() <= 0 || timeout.Seconds() > 3600 {
		timeout = 60 * time.Second
	}

	c.timeout = timeout
	c.newHttpClient()
}

// Set max idle conns for the default client
func SetMaxIdleConns(maxIdleConns int) {
	client.SetMaxIdleConns(maxIdleConns)
}

// Set timeout for default client.
func SetTimeout(timeout time.Duration) {
	client.SetTimeout(timeout)
}

func (c *Client) newHttpClient() {
	c.httpClient = &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				c, err := net.DialTimeout(netw, addr, c.timeout)
				if err != nil {
					return nil, err
				}
				//c.SetDeadline(deadline)
				return c, nil
			},
			ResponseHeaderTimeout: c.timeout,
			MaxIdleConnsPerHost:   c.maxIdleConns,
		},
	}
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

// Data structure returned from the API.
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

func (c *Client) GetCached(r *Request) (retresp *Response, reterr error) {
	resp := &Response{}

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
	return resp, err
}

func MakeID() string {
	buf := make([]byte, 5)
	io.ReadFull(rand.Reader, buf)
	return fmt.Sprintf("%x", buf)
}

// Perform a request, usually called by the request itself.
// User friendly error is enclosed in the response, returned error should be
// for internal use only.
func (c *Client) Do(r *Request) (retresp *Response, reterr error) {
	resp := &Response{}

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
		DebugLog.Printf("Got Request, but we're currently panicing until %s", c.panicUntil.Format(sqlDateTime))
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
			resp.Data = SynthesizeAPIError(500, "APIProxy Error: "+reterr.Error(), 5*time.Minute)
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
			DebugLog.Printf("Error Connecting to API, retrying: %s", err)
			time.Sleep(3 * time.Second)
			continue
		}
		defer httpResp.Body.Close()

		resp.HTTPCode = httpResp.StatusCode

		// We're going to do this asynchronously so we can time it out, AAAAAAA
		type ioRead struct {
			body []byte
			err  error
		}

		readBodyChan := make(chan ioRead)
		go func() {
			bytes, err := ioutil.ReadAll(httpResp.Body)
			readBodyChan <- ioRead{bytes, err}
			close(readBodyChan)
		}()

		select {
		case readBody := <-readBodyChan:
			err = readBody.err
			data = readBody.body
		case <-time.After(c.timeout):
			data = nil
			err = fmt.Errorf("read timed out after %f seconds", c.timeout.Seconds())

			// if ioutil ever does come back, let's handle it.
			go func() {
				id := MakeID()
				DebugLog.Printf("zombie body read %s: %s ? %s", id, r.url, formValues)
				rb := <-readBodyChan
				DebugLog.Printf("zombie read completed %s: %s - %s ? %s\n%s", id, rb.err, r.url, formValues, rb.body)
			}()
		}
		if err != nil {
			DebugLog.Printf("Error Reading from API(%s), retrying...", err)
			time.Sleep(3 * time.Second)
			continue
		}

		break
		log.Printf("WARNING MAJOR REGRESSION: This should NEVER appear.")
	}
	if err != nil {
		DebugLog.Printf("Failed to access api, giving up: %s - %#v", c.BaseURL+r.url, formValues)
		return resp, ErrNetwork
	}

	//data = SynthesizeAPIError(904, "Your IP address has been temporarily blocked because it is causing too many errors. See the cacheUntil timestamp for when it will be opened again. IPs that continuall    y cause a lot of errors in the API will be permanently banned, please take measures to minimize problematic API calls from your application.", time.Second*30)

	// Get cache directive, bail with an error if anything is wrong with XML or
	// time format.  If these produce an error the rest of the data should be
	// considered worthless.
	var cR cacheResp
	err = xml.Unmarshal(data, &cR)
	if err != nil {
		DebugLog.Printf("XML Error: %s", err)
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
