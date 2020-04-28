package requester

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/google/uuid"
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

var lost = sync.Map{}

var reqSocket *zmq.Socket
var handshakeSocket *zmq.Socket

var running bool

/*
Returns the current time given by Batsim, in milliseconds
*/
func RequestTime() int64 {
	id := uuid.New()
	m := &Message{
		RequestType: "time",
		Data:        id.String(),
	}
	r := send(m)
	t, _ := strconv.ParseFloat(r.Data, 64) // in seconds
	t *= 1e3

	return int64(t)
}

func RequestTimer(durationSeconds int64) int64 {
	id := uuid.New()
	m := &Message{
		RequestType: "timer",
		Data:        id.String(),
	}
	r := send(m)
	t, _ := strconv.ParseFloat(r.Data, 64) // in seconds
	t *= 1e3

	return int64(t)
}

func send(m *Message) *Message {
	// Could it be that two tests happen at the exact same time thus
	// calling run() twice?
	// TODO secure run() call?
	if !running {
		go run()
	}
	req <- m
	r := <-res
	// sync maps is the only functionnal way I found to retrieve the
	// right reply.
	// A better solution using channels is very welcome.
	// IDEA : generalize this map idea to store channels associated with uuids
	if r.Data != m.Data {
		fmt.Printf("sent %s, got %s\n", m.Data[:5], r.Data[:5])
		lost.Store(r.Data, r)
		ok := false
		var v interface{}
		for !ok {
			v, ok = lost.Load(m.Data)
		}
		r = v.(*Message)
	}
	if r.Data != m.Data {
		panic("nope")
	}

	fmt.Printf("got %s\n", m.Data[:5])
	return r
}

func run() {
	// This has to be a loop, otherwise not leaving any message behind
	// becomes very tricky.
	running = true
	for {
		// Some solution to the sync problem with batkube.
		// Requests are sent only when the broker makes us know it's ready.
		if handshakeSocket == nil {
			fmt.Println("Creating new handshake socket in time.go")
			handshakeSocket, _ = zmq.NewSocket(zmq.REP)
			handshakeSocket.Connect("tcp://127.0.0.1:27001")
		}
		readyBytes, _ := handshakeSocket.RecvBytes(0)
		ready := string(readyBytes)
		if ready != "ready" {
			panic(fmt.Sprintf("Failed handshake : Expected %s, got %s", "ready", ready))
		}
		handshakeSocket.SendBytes([]byte("ok"), 0)

		// Using a range implies having to close req and opening it
		// again afterwards, which is prone to panics as some routine
		// could send to req while it is closed.
		// TODO : add a timeout? Do not wait for messages to be populated?
		// Wait a bit to leave the scheduler some time to compute beforehand?
		var closeReq bool
		var messages []*Message
		for !closeReq {
			select {
			case m := <-req:
				messages = append(messages, m)
			default:
				if len(messages) > 0 {
					closeReq = true
				}
			}
		}

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
