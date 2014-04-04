/*
Package ratelimit is a generic rate limiter for handling concurrent requests.

Limits the rate at which events can complete while preventing new requests from
starting that may break that limit.

Usage is fairly simple:

    // Create a new rate limiter, limit to 10 requests over any given minute.
    rl := NewRateLimit(10, time.Minute)

Each task must then call Start() to begin, followed by Finish() when it
completes it's task. Start() and Finish() must be called exactly once by each
task.

    func task(rl *RateLimit) {
        rl.Start(0)
        // Do stuff
        rl.Finish(false)
    }

*/
package ratelimit

import (
	"errors"
	"time"
)

// ErrTimeout indicated a timeout when attempting to begin a task.
var ErrTimeout = errors.New("timeout waiting for clearance to continue")

// ErrAlreadyClosed indicates an attempt to use a closed RateLimit.
var ErrAlreadyClosed = errors.New("already closed")

/*
NewRateLimit will return a new rate limiter that limits to maxEvents events
over any given duration of period length.

Note that the number of concurrent tasks running will never exceed maxEvents.
*/
func NewRateLimit(maxEvents int, period time.Duration) *RateLimit {
	var rl RateLimit

	rl.start = make(chan struct{})
	rl.finish = make(chan bool, maxEvents*2)
	rl.close = make(chan chan error)

	rl.events = make(map[time.Time]struct{}, maxEvents)

	rl.maxEvents = maxEvents
	rl.period = period

	rl.activeStart = rl.start

	go rl.run()

	return &rl
}

/*
Start should be called at the beginning of a task. It will block as needed in
order to ensure the rate remains below the specified limit.

A timeout can be specified which will cause Start to return ErrTimeout if the
task is not allowed to begin within that time.  A timeout of 0 will never
time out.
*/
func (rl *RateLimit) Start(timeout time.Duration) (retErr error) {
	// Use recover to avoid panicing the entire program should start be called
	// on a closed RateLimit.
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(error)
			if !ok || e == nil {
				panic(r)
			}

			if e.Error() == "runtime error: send on closed channel" {
				retErr = ErrAlreadyClosed
			} else {
				retErr = e
			}
		}
	}()

	var timeoutChan <-chan time.Time
	if timeout != 0 {
		timeoutChan = time.After(timeout)
	}

	select {
	case <-timeoutChan:
		return ErrTimeout

	case rl.start <- struct{}{}:
		return nil
	}
}

/*
Finish is used by a task to signal its completion. It will never block.

skip is used to determine whether or not this task will mark an event. If skip
is true, the event will not count towards the rate limiting.
*/
func (rl *RateLimit) Finish(skip bool) (retErr error) {
	// Use recover to avoid panicing the entire program should start be called
	// on a closed RateLimit.
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(error)
			if !ok || e == nil {
				panic(r)
			}

			if e.Error() == "runtime error: send on closed channel" {
				DebugLog.Printf("Already closed: %s", e)
				retErr = ErrAlreadyClosed
			} else {
				DebugLog.Printf("Other Error: %s", e)
				retErr = e
			}
		}
	}()

	rl.finish <- skip

	return nil
}

/*
Close the rate limiter, cleaning up any resources in use.
*/
func (rl *RateLimit) Close() (retErr error) {
	// Use recover to avoid panicing the entire program should start be called
	// on a closed RateLimit.
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(error)
			if !ok || e == nil {
				panic(r)
			}

			if e.Error() == "runtime error: send on closed channel" {
				DebugLog.Printf("Already closed: %s", e)
				retErr = ErrAlreadyClosed
			} else {
				DebugLog.Printf("Other Error: %s", e)
				retErr = e
			}
		}
	}()

	respChan := make(chan error)
	rl.close <- respChan
	err := <-respChan

	return err
}
