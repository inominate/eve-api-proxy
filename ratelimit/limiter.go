package ratelimit

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"
)

/* DebugLog be set up with a logger for debugging purposes. */
var DebugLog = log.New(ioutil.Discard, "", 0)

/*
RateLimit should only be created using NewRateLimit()
*/
type RateLimit struct {
	maxEvents int
	period    time.Duration

	outstanding  int
	expireEvents <-chan time.Time
	events       map[time.Time]struct{}
	nextExpire   time.Time

	// activeStart is what we nil out when we need to block new requests
	activeStart chan struct{}

	// start keeps our real start channel.
	start  chan struct{}
	finish chan bool
	close  chan chan error
	count  chan chan int
}

/* countEvents should only ever called by run, dangerous if used elsewhere. */
func (rl *RateLimit) countEvents() (eventCount int) {
	var nextExpire time.Time
	now := time.Now()

	for t := range rl.events {
		if t.Before(now) {
			delete(rl.events, t)
		} else {
			eventCount++

			if nextExpire.IsZero() || t.Before(nextExpire) {
				nextExpire = t
			}
		}
	}

	if nextExpire.IsZero() {
		rl.expireEvents = nil
	} else if nextExpire != rl.nextExpire {
		// Don't create new timers unless we actually need to.
		rl.nextExpire = nextExpire
		rl.expireEvents = time.After(nextExpire.Sub(now))
	}
	return
}

/* addEvent should only ever called by run, dangerous if used elsewhere. */
func (rl *RateLimit) addEvent() {
	rl.events[time.Now().Add(rl.period)] = struct{}{}
}

/*
run is the main handler, started by NewRateLimit and should never be used
elsewhere
*/
func (rl *RateLimit) run() {
runLoop:
	for {
		select {
		case <-rl.expireEvents:
			rl.runExpire()

		case skip := <-rl.finish:
			rl.runFinish(skip)

		case <-rl.activeStart:
			rl.runStart()

		case respChan := <-rl.count:
			respChan <- rl.countEvents()

		case respChan := <-rl.close:
			rl.runClose(respChan)
			break runLoop
		}
	}

	DebugLog.Printf("Worker cleanup complete, shutting down.")
}

/*  runExpire is used by run to expire events on a timer. */
func (rl *RateLimit) runExpire() {
	count := rl.countEvents()
	DebugLog.Printf("Expired events, have %d events remaining.", count)

	if rl.outstanding+count < rl.maxEvents {
		DebugLog.Printf("Event limit clear, continuing")
		rl.activeStart = rl.start
	}
}

/* runFinish is used by run to handle the completion of a task, marking an event */
func (rl *RateLimit) runFinish(skip bool) {
	count := rl.countEvents()

	if skip {
		DebugLog.Printf("Event finished, but going uncounted.")
	} else {
		rl.addEvent()
		count++

		DebugLog.Printf("Event finished, current count is %d.", count)
		if count >= rl.maxEvents {
			// Stop listening for new start requests.
			rl.activeStart = nil

			DebugLog.Printf("Event limit reached, blocking start requests.")
		}
	}

	rl.outstanding--
	if rl.outstanding+count < rl.maxEvents {
		DebugLog.Printf("Event limit clear, accepting new start requests.")
		rl.activeStart = rl.start
	}
}

/* runStart is used by run to handle the beginning of an event. */
func (rl *RateLimit) runStart() {
	count := len(rl.events)

	rl.outstanding++
	if rl.outstanding+count == rl.maxEvents {
		// Stop listening for start requests causing new ones to block until
		// some existing events finish.
		rl.activeStart = nil

		DebugLog.Printf("New requests could break error limit, slowing down.")
	} else if rl.outstanding+count > rl.maxEvents {
		log.Printf("New requests have broken error limit, this shouldn't happen. %d+%d (%d) > %d", rl.outstanding, count, rl.outstanding+count, rl.maxEvents)
	}
}

/* runClose is used by run to handle the dirty work of shutting down */
func (rl *RateLimit) runClose(respChan chan error) {
	close(rl.close)
	close(rl.start)
	close(rl.finish)
	close(rl.count)

	var err error
	if rl.outstanding > 0 {
		err = fmt.Errorf("error closing, %d events still rl.outstanding", rl.outstanding)
	}

	respChan <- err
}
