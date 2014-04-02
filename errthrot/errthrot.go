package errthrot

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"time"
)

// Set up a logger that can be turned on for debugging.
var DebugLog = log.New(ioutil.Discard, "", 0)

type ErrThrot struct {
	maxErrors int
	period    time.Duration

	expireErrors <-chan time.Time
	errors       map[time.Time]struct{}

	start  chan bool
	finish chan error
	close  chan chan error
}

/* countErrors should only ever called by run, dangerous if used elsewhere. */
func (e *ErrThrot) countErrors() (errorCount int) {
	var nextExpire time.Time
	now := time.Now()

	for t := range e.errors {
		if t.Before(now) {
			delete(e.errors, t)
		} else {
			errorCount++
		}
		if nextExpire.IsZero() || t.Before(nextExpire) {
			nextExpire = t
		}
	}

	if nextExpire.IsZero() {
		e.expireErrors = nil
	} else {
		e.expireErrors = time.After(nextExpire.Sub(now))
	}
	return
}

/* addError() should only ever called by run, dangerous if used elsewhere. */
func (e *ErrThrot) addError() {
	e.errors[time.Now().Add(e.period)] = struct{}{}
}

func (e *ErrThrot) run() {
	var count, outstanding int
	var startChan = e.start

	for {
		select {
		case err := <-e.finish:
			DebugLog.Printf("PreEnd:	O: %d	E: %d	T: %d", outstanding, count, outstanding+count)

			if err == nil {
				DebugLog.Printf("Item finished with no error.")
			} else {
				e.addError()
				count = e.countErrors()

				DebugLog.Printf("Item finished with error. Current error count is %d.", count)
				if count >= e.maxErrors {
					DebugLog.Printf("Error limit reached, blocking start requests.")

					// Stop listening for new start requests.
					startChan = nil
				}
			}

			outstanding--
			if outstanding+count < e.maxErrors {
				DebugLog.Printf("Error limit clear, continuing")
				startChan = e.start
			}
			DebugLog.Printf("PostEnd:	O: %d	E: %d	T: %d", outstanding, count, outstanding+count)

		case <-startChan:
			DebugLog.Printf("PreStart:	O: %d	E: %d	T: %d", outstanding, count, outstanding+count)
			outstanding++
			DebugLog.Printf("New Item Starting: %d.", outstanding)
			if outstanding+count == e.maxErrors {
				DebugLog.Printf("New requests could break error limit, slowing down.")
				// Stop listening for start requests causing new ones to block until
				// some existing tasks finish.
				startChan = nil
			} else if outstanding+count > e.maxErrors {
				log.Printf("New requests have broken error limit, this shouldn't happen. %d+%d (%d) > %d", outstanding, count, outstanding+count, e.maxErrors)
			}
			DebugLog.Printf("PostStart:	O: %d	E: %d	T: %d", outstanding, count, outstanding+count)

		case respChan := <-e.close:
			DebugLog.Printf("Starting worker shutdown")

			close(e.close)
			close(e.start)
			close(e.finish)

			var err error
			if outstanding > 0 {
				err = fmt.Errorf("error closing, %d tasks still outstanding", outstanding)
			}

			respChan <- err

			DebugLog.Printf("Worker cleanup complete")
			return
		}
	}
}

func NewErrThrot(maxErrors int, period time.Duration) *ErrThrot {
	var e ErrThrot
	e.start = make(chan bool)
	e.finish = make(chan error, maxErrors*5)
	e.close = make(chan chan error)

	e.maxErrors = maxErrors
	e.period = period
	go e.run()
	return &e
}

var ErrTimeout = errors.New("timeout waiting for clearance to continue")
var ErrAlreadyClosed = errors.New("already closed")

func (e *ErrThrot) Start(timeout time.Duration) (retErr error) {
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

	var timeoutChan <-chan time.Time
	if timeout != 0 {
		timeoutChan = time.After(timeout)
	}

	select {
	case <-timeoutChan:
		return ErrTimeout

	case e.start <- true:
		return nil
	}
}

func (e *ErrThrot) Finish(err error) (retErr error) {
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

	e.finish <- err

	return nil
}

func (e *ErrThrot) Close() (retErr error) {
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
	e.close <- respChan
	err := <-respChan

	return err
}
