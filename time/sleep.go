// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import (
	"fmt"

	"github.com/oar-team/batsky-go/requester"
)

// Values for the timer status field.
const (
	// Timer has no status set yet.
	timerNoStatus = iota

	// Waiting for timer to fire.
	// The timer is in some P's heap.
	timerWaiting

	// Running the timer function.
	// A timer will only have this status briefly.
	timerRunning

	// The timer is deleted and should be removed.
	// It should not be run, but it is still in some P's heap.
	timerDeleted

	// The timer is being modified.
	// The timer will only have this status briefly.
	timerModifying

	// The timer has been modified
	// This concatenas the modifiedEarlier and modifiedLater cases
	// from runtime
	timerModified
)

// Interface to timers implemented in package runtime.
// Must be in sync with ../runtime/time.go:/^type timer
// Note : has been modified for the custom time lib
type runtimeTimer struct {
	when        int64
	f           func(interface{})
	arg         interface{}
	currentTime *Time
	status      uint32
}

// Sleep pauses the current goroutine for at least the duration d.
// A negative or zero duration causes Sleep to return immediately.
func Sleep(d Duration) {
	<-NewTimer(d).C
}

// when is a helper function for setting the 'when' field of a runtimeTimer.
// It returns what the time will be, in nanoseconds, Duration d in the future.
// If d is negative, it is ignored. If the returned value would be less than
// zero because of an overflow, MaxInt64 is returned.
func when(d Duration) int64 {
	if d <= 0 {
		return runtimeNano()
	}
	t := requester.RequestTimer(int64(d)) + int64(d)
	if t < 0 {
		t = 1<<63 - 1 // math.MaxInt64
	}
	return t
}

func nanoToTime(t int64) Time {
	// This line must be synced with what now() returns
	sec, nsec, mono := t/1e9, int32(t%1e9), t
	mono -= startNano
	sec += unixToInternal - minWall
	if uint64(sec)>>33 != 0 {
		return Time{uint64(nsec), sec + minWall, Local}
	}
	return Time{hasMonotonic | uint64(sec)<<nsecShift | uint64(nsec), mono, Local}
}

// maxWhen is the maximum value for timer's when field.
const maxWhen = 1<<63 - 1

func startTimer(t *runtimeTimer) {
	if t.status != timerNoStatus {
		panic("startTimer called with initialized timer")
	}
	t.status = timerWaiting
	go func() {
		for {
			currentTime := runtimeNano()
			switch t.status {
			case timerWaiting:
				fmt.Printf("timer: currentTime %d; when %d\n", currentTime, t.when)
				if currentTime >= t.when {
					fmt.Println("gotta run")
					*t.currentTime = nanoToTime(currentTime)
					t.status = timerRunning
				}
			case timerRunning:
				t.f(t.arg)
				t.status = timerDeleted
			case timerDeleted:
				fmt.Printf("timer finished: %d\n", currentTime)
				return
			case timerModifying:
				for t.status == timerModifying {
				}
			default:
				panic("bad timer")
			}
		}
	}()
}

// stopTimer stops a timer.
// It reports whether t was stopped before being run.
func stopTimer(t *runtimeTimer) bool {
	switch s := t.status; s {
	case timerWaiting:
		t.status = timerDeleted
		return true
	case timerNoStatus, timerDeleted:
		return false
	case timerRunning, timerModifying:
		// Timer is being run or there is a simultaneous call to modTimer.
		// We wait for those calls to end
		for s == timerRunning || s == timerModifying {
		}
		t.status = timerDeleted
		return true
	default:
		panic("bad timer")
	}
}

// resettimer resets the time when a timer should fire.
// If used for an inactive timer, the timer will become active.
// This should be called instead of addtimer if the timer value has been,
// or may have been, used previously.
// Reports whether the timer was modified before it was run.
func resetTimer(t *runtimeTimer, when int64) bool {
	return modTimer(t, when, t.f, t.arg)
}

// modtimer modifies an existing timer.
// Reports whether the timer was modified before it was run.
func modTimer(t *runtimeTimer, when int64, f func(interface{}), arg interface{}) bool {
	if when < 0 {
		when = maxWhen
	}

	var pending bool

	switch s := t.status; s {
	case timerWaiting:
		t.status = timerDeleted
		pending = true
	case timerNoStatus, timerDeleted:
		pending = false
	case timerRunning, timerModifying:
		for s == timerRunning || s == timerModifying {
		}
		pending = false
	default:
		panic("bad timer")
	}

	t.status = timerModifying

	t.f = f
	t.arg = arg
	t.when = when

	t.status = timerNoStatus
	startTimer(t)

	return pending
}

// The Timer type represents a single event.
// When the Timer expires, the current time will be sent on C,
// unless the Timer was created by AfterFunc.
// A Timer must be created with NewTimer or AfterFunc.
type Timer struct {
	C <-chan Time
	r runtimeTimer
}

// Stop prevents the Timer from firing.
// It returns true if the call stops the timer, false if the timer has already
// expired or been stopped.
// Stop does not close the channel, to prevent a read from the channel succeeding
// incorrectly.
//
// To ensure the channel is empty after a call to Stop, check the
// return value and drain the channel.
// For example, assuming the program has not received from t.C already:
//
// 	if !t.Stop() {
// 		<-t.C
// 	}
//
// This cannot be done concurrent to other receives from the Timer's
// channel or other calls to the Timer's Stop method.
//
// For a timer created with AfterFunc(d, f), if t.Stop returns false, then the timer
// has already expired and the function f has been started in its own goroutine;
// Stop does not wait for f to complete before returning.
// If the caller needs to know whether f is completed, it must coordinate
// with f explicitly.
func (t *Timer) Stop() bool {
	if t.r.f == nil {
		panic("time: Stop called on uninitialized Timer")
	}
	return stopTimer(&t.r)
}

// NewTimer creates a new Timer that will send
// the current time on its channel after at least duration d.
func NewTimer(d Duration) *Timer {
	c := make(chan Time, 1)
	t := &Timer{
		C: c,
		r: runtimeTimer{
			when: when(d),
			f:    sendTime,
		},
	}
	t.r.currentTime = &Time{}
	t.r.arg = sendTimeArgs{c, t.r.currentTime}
	startTimer(&t.r)
	return t
}

// Reset changes the timer to expire after duration d.
// It returns true if the timer had been active, false if the timer had
// expired or been stopped.
//
// Reset should be invoked only on stopped or expired timers with drained channels.
// If a program has already received a value from t.C, the timer is known
// to have expired and the channel drained, so t.Reset can be used directly.
// If a program has not yet received a value from t.C, however,
// the timer must be stopped and—if Stop reports that the timer expired
// before being stopped—the channel explicitly drained:
//
// 	if !t.Stop() {
// 		<-t.C
// 	}
// 	t.Reset(d)
//
// This should not be done concurrent to other receives from the Timer's
// channel.
//
// Note that it is not possible to use Reset's return value correctly, as there
// is a race condition between draining the channel and the new timer expiring.
// Reset should always be invoked on stopped or expired channels, as described above.
// The return value exists to preserve compatibility with existing programs.
func (t *Timer) Reset(d Duration) bool {
	if t.r.f == nil {
		panic("time: Reset called on uninitialized Timer")
	}
	w := when(d)
	return resetTimer(&t.r, w)
}

type sendTimeArgs struct {
	c chan Time
	t *Time
}

func sendTime(args interface{}) {
	// Non-blocking send of time on c.
	// Used in NewTimer, it cannot block anyway (buffer).
	// Used in NewTicker, dropping sends on the floor is
	// the desired behavior when the reader gets behind,
	// because the sends are periodic.
	select {
	case args.(sendTimeArgs).c <- *args.(sendTimeArgs).t:
	default:
	}
}

// After waits for the duration to elapse and then sends the current time
// on the returned channel.
// It is equivalent to NewTimer(d).C.
// The underlying Timer is not recovered by the garbage collector
// until the timer fires. If efficiency is a concern, use NewTimer
// instead and call Timer.Stop if the timer is no longer needed.
func After(d Duration) <-chan Time {
	return NewTimer(d).C
}

// AfterFunc waits for the duration to elapse and then calls f
// in its own goroutine. It returns a Timer that can
// be used to cancel the call using its Stop method.
func AfterFunc(d Duration, f func()) *Timer {
	t := &Timer{
		r: runtimeTimer{
			when: when(d),
			f:    goFunc,
			arg:  f,
		},
	}
	startTimer(&t.r)
	return t
}

func goFunc(arg interface{}) {
	go arg.(func())()
}
