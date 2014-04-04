package ratelimit

import (
	"log"
	"os"
	"testing"
	"time"
)

func TaskClean(t *testing.T, e *RateLimit, id int, timeout time.Duration, taskLength time.Duration) bool {
	t.Logf("Clean task %d starting.", id)

	err := e.Start(timeout)
	if err != nil {
		t.Logf("Clean task %d failed to start: %s", id, err)
		return true
	}

	t.Logf("Clean task %d started.", id)
	time.Sleep(taskLength)

	t.Logf("Clean task %d finishing.", id)
	e.Finish(true)
	t.Logf("Clean task %d finished.", id)

	return false
}

func TaskError(t *testing.T, e *RateLimit, id int, timeout time.Duration, taskLength time.Duration) bool {
	t.Logf("Error task %d starting.", id)

	err := e.Start(timeout)
	if err != nil {
		t.Logf("Clean task %d failed to start: %s", id, err)
		return true
	}

	t.Logf("Error task %d started.", id)
	time.Sleep(taskLength)

	t.Logf("Error task %d finishing.", id)
	e.Finish(false)
	t.Logf("Error task %d finished.", id)

	return false
}

func Test_Clean(t *testing.T) {
	et := NewRateLimit(5, 1*time.Second)
	taskLength := 1 * time.Millisecond

	successChan := make(chan bool)
	for i := 1; i <= 20; i++ {
		go func(i int) {
			TaskClean(t, et, i, 0, taskLength)
			successChan <- true
		}(i)
	}

	timeout := time.After(100 * time.Millisecond)
	for i := 1; i <= 20; i++ {
		select {
		case <-successChan:
		case <-timeout:
			t.Errorf("Clean tasks failed to complete.")
			return
		}
	}

	t.Logf("Clean test succeeded.")
}

func Test_Error(t *testing.T) {
	et := NewRateLimit(4, 100*time.Millisecond)
	taskLength := 5 * time.Millisecond
	begin := time.Now()
	minDuration := 200 * time.Millisecond

	successChan := make(chan bool)
	timeout := time.After(2 * time.Second)
	go func() {
		for i := 1; i <= 20; i++ {
			if i%2 == 0 {
				TaskClean(t, et, i, 0, taskLength)
			} else {
				TaskError(t, et, i, 0, taskLength)
			}
		}
		successChan <- true
	}()

	select {
	case <-successChan:
		if time.Since(begin) < minDuration {
			t.Errorf("tasks completed but error throttling not functional")
		} else {
			t.Logf("Error test succeeded.")
		}
		return
	case <-timeout:
		t.Errorf("Error tasks failed to complete.")
		return
	}
}

func Test_Timeout(t *testing.T) {
	et := NewRateLimit(5, 10*time.Second)
	taskLength := 10 * time.Millisecond

	successChan := make(chan bool)
	timeout := time.After(time.Second)
	go func() {
		for i := 1; i <= 5; i++ {
			TaskError(t, et, i, 0, taskLength)
		}
		time.Sleep(10 * time.Millisecond)
		successChan <- TaskClean(t, et, -1, 10*time.Millisecond, taskLength)
	}()

	select {
	case success := <-successChan:
		if success {
			t.Logf("Timeout test succeeded.")
		} else {
			t.Errorf("tasks completed but failed to time out")
		}
		return
	case <-timeout:
		t.Errorf("Timeout tasks failed to complete.")
		return
	}
}

func Test_Close(t *testing.T) {
	t.Logf("Close test")
	et := NewRateLimit(4, 10*time.Millisecond)
	taskLength := 1 * time.Millisecond

	successChan := make(chan bool)
	timeout := time.After(2 * time.Second)
	go func() {
		for i := 1; i <= 10; i++ {
			if i%2 == 0 {
				TaskClean(t, et, i, 0, taskLength)
			} else {
				TaskError(t, et, i, 0, taskLength)
			}
		}
		successChan <- true
	}()

	select {
	case <-successChan:
	case <-timeout:
		t.Errorf("close tasks failed to complete.")
	}

	t.Logf("First close")
	err := et.Close()
	if err != nil {
		t.Errorf("error closing et: %s", err)
	}

	t.Logf("Second close")
	err = et.Close()

	if err == ErrAlreadyClosed {
		t.Logf("successfully got error trying second close")
	} else {
		t.Errorf("error closing et: %s", err)
	}

	go func() {
		err := et.Start(0)
		if err != ErrAlreadyClosed {
			t.Errorf("start gave unknown error: %s", err)
		}
		successChan <- true
	}()
	select {
	case <-successChan:
	case <-time.After(100 * time.Millisecond):
		t.Errorf("start failed to detect closed channel, timed out")
	}

	go func() {
		err := et.Finish(true)
		if err != ErrAlreadyClosed {
			t.Errorf("finish gave unknown error: %s", err)
		}
		successChan <- true
	}()
	select {
	case <-successChan:
	case <-time.After(100 * time.Millisecond):
		t.Errorf("finish failed to detect closed channel, timed out")
	}
}

/*
Test speculative error checking, if we're near the error limit don't allow
concurrent requests to pile up.
*/
func Test_Speculate(t *testing.T) {
	et := NewRateLimit(3, 100*time.Millisecond)

	successChan := make(chan bool)
	timeout := time.After(120 * time.Millisecond)

	begin := time.Now()
	// trigger two quick errors followed by a long, non-error task
	go func() {
		TaskError(t, et, 1, 0, 0)
		TaskError(t, et, 2, 0, 0)
		successChan <- true
		TaskClean(t, et, 3, 0, 200*time.Millisecond)
	}()

	select {
	case <-successChan:
	case <-timeout:
		t.Errorf("speculate setup tasks failed to complete.")
		return
	}

	TaskClean(t, et, -1, 0, 0)
	if time.Since(begin) < 100*time.Millisecond {
		t.Errorf("speculate not holding up tasks")
	} else {
		t.Logf("speculate held up as expected")
	}

	begin = time.Now()
	TaskClean(t, et, -1, 0, 0)
	if time.Since(begin) > 10*time.Millisecond {
		t.Errorf("speculate incorrectly holding up tasks")
	} else {
		t.Logf("speculate stopped as expected")
	}

}

func init() {
	DebugLog = log.New(os.Stdout, "ratelimit	", log.LstdFlags|log.Lshortfile)
}
