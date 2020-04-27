package main

import (
	. "github.com/oar-team/batsky-go/time"
)

// This file centralises the requests that have to be redirected to
// batkube.

// Requests are split up into two categories : requests for the current time
// and requests for timers. Knowing when a scheduler asks for a timer is
// important in order to send CALL_ME_LATER events to Batsim.
//
// data for a time request is ""
// data for a timer request is the duration of the timer

type requestMessage struct {
	requestType string
	data        string
}

var req chan *requestMessage
var res chan *requestMessage

func requestTime() int64 {
	return 0
}

func requestTimer(d Duration) {
}

func main() {

}
