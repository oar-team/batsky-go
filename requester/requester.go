package requester

import (
	"fmt"
	realtime "time"

	zmq "github.com/pebbe/zmq4"
)

// This file centralises the requests that have to be redirected to
// batkube.

// Requests are split up into two categories : requests for the current time
// and requests for timers. Knowing when a scheduler asks for a timer is
// important in order to send CALL_ME_LATER events to Batsim.
//
// data for a time request is ""
// data for a timer request is the duration of the timer

type message struct {
	requestType string
	data        string
}

//var closeReq = make(chan bool)
var req = make(chan *message)
var res = make(chan *message)
var done = make(chan bool)

var reqSocket *zmq.Socket

var sending bool

func RequestTime() int64 {
	m := &message{
		requestType: "time",
		data:        "",
	}
	r := send(m)
	if r != m {
		fmt.Printf("Expected %p, found %p\n", m, r)
	}
	return 0
}

func RequestTimer(durationSeconds int64) {
	m := &message{
		requestType: "timer",
		data:        "0",
	}
	r := send(m)
	if r != m {
		fmt.Printf("Expected %p, found %p\n", m, r)
	}
}

func send(m *message) *message {
	// Could it be that two tests happen at the exact same time thus
	// calling run() twice?
	// TODO secure run() call?
	var r *message
	fmt.Println("send")
	if sending {
		req <- m
		r = <-res
	} else {
		// If doSend is not running yet, this call has the
		// responsibility of running it and waiting for it to end
		// We have to check here when all res have be sent so as not
		// to call another instance of it.
		sending = true
		go doSend()
		req <- m
		r = <-res
		<-done
		sending = false
	}
	return r
}

func doSend() {
	fmt.Println("running")
	var messages []*message

	var closeReq bool
	// Temporary, leave some time for things to arrive
	go func() {
		realtime.Sleep(1 * realtime.Second)
		closeReq = true
	}()

	// Using a range implies having to close req and opening it again after
	// the loop, which is prone to panics as some routine could send to req
	// while it is closed.
	for !closeReq {
		select {
		case m := <-req:
			messages = append(messages, m)
		default:
		}
	}

	if reqSocket == nil {
		fmt.Println("creating new request socket in time.go")
		reqSocket, _ = zmq.NewSocket(zmq.REQ)
		reqSocket.Connect("tcp://127.0.0.1:27000")
	}
	//msg, err := json.Marshal(messages)
	//if err != nil {
	//	panic(fmt.Sprintf("Error marshaling message %v:", messages) + err.Error())
	//}
	//_, err = reqSocket.SendBytes(msg, 0)
	//if err != nil {
	//	panic("Error sending message: " + err.Error())
	//}

	//messages = nil
	//reply, err := reqSocket.RecvBytes(0)
	//if err != nil {
	//	panic("Error receiving message:" + err.Error())
	//}
	//if err = json.Unmarshal(reply, &messages); err != nil {
	//	panic("Could not unmarshal data:" + err.Error())
	//}

	for _, message := range messages {
		// TODO : some logic to send in the correct order
		res <- message
	}
	done <- true
}
