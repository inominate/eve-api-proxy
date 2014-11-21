# EVE-Online API Proxy #

This is a simple proxy intended for high volume access of the EVE Online API by
scripts and programs too trivial to justify dealing with CCP's caching rules.

### Features ###
* The proxy deals with the housekeeping of dealing with the EVE API. Including
following caching rules, dealing with connection issues, etc. This allows
simple scripts to hammer away without worrying about such things.

* All incoming requests are fed through a worker pool, limiting the number of
concurrent connections. The worker pool is pipelined to reduce the number
of connections being made.  This also effectively rate limits the proxy,
allowing many applications on the same host to avoid overloading the API.

* Identical requests from different applications will share the same cache,
even if they're using different HTTP methods or parameters. Note that this
isn't hardened against malicious requests.

* APIKeyInfo.xml.aspx likes to throw error code 221s for no apparent reason,
the proxy will correct for them. Workarounds for other issues can be added
fairly easily.

* Locations.xml.aspx will fail completely when given a list of item ids and one
or more are invalid. This can occur due to the cache lag in other endpoints 
still showing nonexistent items. The proxy can correct for this faster than
trying each id independently. 

* If for some reason the API throws a temp ban, the proxy will refuse to
continue servicing requests until the ban expires.

* Can be used to log API requests being made for debugging purposes.

* All API requests are done through https and will use compression if possible.

### Before Installation ####
A working go installation is required. See: http://golang.org/doc/install

If you do not yet have a go workspace created, you will need to set your GOPATH
to point to one. 

``` bash
mkdir ~/go
export GOPATH=~/go
```

### Running the Proxy ####
To install, build, and run the proxy:

``` bash
go get github.com/inominate/eve-api-proxy
cd $GOPATH/bin
./eve-api-proxy -create
mv apiproxy.xml.default apiproxy.xml 
./eve-api-proxy
```

The proxy binds to localhost:3748 by default.  Applications wishing to use it
can simply point to http://localhost:3748/ instead of 
https://api.eveonline.com/. Runtime statistics are available at "/stats".

Caution should be used in exposing the proxy to the outside world. I
recommend putting it behind a webserver such as nginx that is configured to
only allow requests from authorized IP addresses.

The only difference is that the proxy adds a new api error code 500 with HTTP
code 504, to indicate an inability to connect to the API.  

### Configuration File ###

##### `Listen`
In the form of ip:port, or :port to listen on all interfaces.  Currently only
supports a single bind point.  Default is localhost:3748.

##### `Threads`
Sets the number of operating system threads to run simultaneously. This is an
internal Go setting and unrelated to the number of simultaneous workers. A
setting of 0 will use one thread per available logical CPU. This should
virtually never be anything other than 0 or 1.  Default is 0.

##### `Workers`
Number of workers to run processing requests, each worker tries to maintain its
own semi-permanent connection to the API. Default is 10.

##### `Retries`
The number of times to retry the API in case of a connection issue. Default is
3.

##### `APITimeout`
The maximum length of time in seconds that any single request to the API should
take. Default is 60 seconds and can be increased in the case of slow connections 
pulling large blocks of XML.

##### `RequestsPerSecond`
The maximum number of requests per second that will be sent to the Eve API.
According to CCP FoxFour this should be kept at 30 or below.

Default is 30.

##### `ErrorPeriod`
The length of time to consider API errors in the count. Higher numbers allow
bursty errors to run without interruption but can cause timeouts when the limit
is reached.  Lower numbers may cause throttling sooner but can allow for 
results to be returned before timeouts.

If a request is held up for more than 30 seconds, the proxy will return a CCP
style API Error 500.

Default is 180 seconds or three minutes.

##### `MaxErrors`
The maximum number of errors that can occur over any given ErrorPeriod. This
number should always be greater than the number of workers or else workers will
be held up in order go guarantee the limit is not broken. Default is 250.

According to CCP FoxFour, the CCP API will issue a temp ban at 300 errors over
three minutes.

##### `CacheDir`
The directory in which cached API data will be stored.  Default is ./cache/

##### `FastStart`
Fast start mode will clear the cache on startup instead of reloading it. As long
as you're not restarting too often this can be left on.

##### `LogFile`
File to use for general logging. Default is blank and will use stdout.

##### `LogRequests`
Log all requests instead of just reporting problems. Default is false.

##### `CensorLog`
Remove most of the vCode from the logs for privacy reasons. Default is true.

##### `Debug`
Enable debugging logging. Default is false.

##### `DebugLogFile`
File to use for debugging. Can be the same as LogFile. Default is blank and will use stdout.

