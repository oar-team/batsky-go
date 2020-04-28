package requester

import (
	"encoding/json"
	"fmt"
	"strconv"

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

type Message struct {
	RequestType string
	Data        string
}

//var closeReq = make(chan bool)
var req = make(chan *Message)
var res = make(chan *Message)
var done = make(chan bool)

var reqSocket *zmq.Socket
var handshakeSocket *zmq.Socket

var running bool

/*
Returns the current time given by Batsim, in milliseconds
*/
func RequestTime() int64 {
	m := &Message{
		RequestType: "time",
		Data:        "",
	}
	r := send(m)
	t, _ := strconv.ParseFloat(r.Data, 64) // in seconds
	t *= 1e3                               // in nanoseconds

	return int64(t)
}

func RequestTimer(durationSeconds int64) {
	m := &Message{
		RequestType: "timer",
		Data:        "0",
	}
	r := send(m)
	if r != m {
		fmt.Printf("Expected %p, found %p\n", m, r)
	}
}

func send(m *Message) *Message {
	// Could it be that two tests happen at the exact same time thus
	// calling run() twice?
	// TODO secure run() call?
	if !running {
		go run()
	}
	req <- m
	return <-res
}

func run() {
	// This has to be a loop, otherwise not leaving any message behind
	// becomes very tricky.
	running = true
	for {
		// Some solution to the sync problem with batkube mentioned
		// thereafter
		if handshakeSocket == nil {
			fmt.Println("Creating new handshake socket in time.go")
			handshakeSocket, _ = zmq.NewSocket(zmq.REP)
			handshakeSocket.Connect("tcp://127.0.0.1:27001")
		}
		readyBytes, _ := handshakeSocket.RecvBytes(0)
		ready := string(readyBytes)
		if ready != "ready" {
			panic(fmt.Sprintf("Expected %s, got %s", "ready", ready))
		}
		handshakeSocket.SendBytes([]byte("ok"), 0)

		// Using a range implies having to close req and opening it
		// again afterwards, which is prone to panics as some routine
		// could send to req while it is closed.
		// TODO : add a timeout?
		var closeReq bool
		var messages []*Message
		for !closeReq {
			select {
			case m := <-req:
				messages = append(messages, m)
			default:
				//if len(messages) > 0 {
				//	closeReq = true
				//}
				closeReq = true
			}
		}
		// Potential issue : requests are sent one simulation step
		// later because of this mechanic. An improvement would be to
		// stop reading from req only when the broker is ready.

		if reqSocket == nil {
			fmt.Println("Creating new request socket in time.go")
			reqSocket, _ = zmq.NewSocket(zmq.REQ)
			reqSocket.Connect("tcp://127.0.0.1:27000")
		}

		msg, err := json.Marshal(messages)
		if err != nil {
			panic(fmt.Sprintf("Error marshaling message %v:", messages) + err.Error())
		}
		_, err = reqSocket.SendBytes(msg, 0)
		if err != nil {
			panic("Error sending message: " + err.Error())
		}

		messages = nil
		reply, err := reqSocket.RecvBytes(0)
		if err != nil {
			panic("Error receiving message:" + err.Error())
		}
		if err = json.Unmarshal(reply, &messages); err != nil {
			panic("Could not unmarshal data:" + err.Error())
		}

		for _, message := range messages {
			// TODO : some logic to send in the correct order
			res <- message
		}
	}
}
