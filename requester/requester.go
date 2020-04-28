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
	UUID        string
}

//var closeReq = make(chan bool)
var req = make(chan *Message)
var res = sync.Map{}

var reqSocket *zmq.Socket
var handshakeSocket *zmq.Socket

var running bool

/*
Returns the current time given by Batsim, in milliseconds
*/
func RequestTime() int64 {
	m := &Message{
		RequestType: "time",
	}
	r := send(m)
	t, _ := strconv.ParseFloat(r.Data, 64) // in seconds
	t *= 1e3

	return int64(t)
}

func RequestTimer(durationSeconds int64) int64 {
	m := &Message{
		RequestType: "timer",
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
		fmt.Println("Running loop for time requests. If this message appears twice, there is a problem in requester.go")
		go run()
	}

	// This is a nice solution allowing us to retrieve the result
	// only when it is ready without blocking the entire code.
	m.UUID = uuid.New().String()
	_, ok := res.Load(m.UUID)
	for ok {
		// This would be very, very unlucky
		fmt.Printf(fmt.Sprintf("Map entry already set for UUID %s. Generating a new one\n", m.UUID))
		m.UUID = uuid.New().String()
		_, ok = res.Load(m.UUID)
	}
	res.Store(m.UUID, make(chan *Message))
	resChan, ok := res.Load(m.UUID)
	if !ok {
		panic(fmt.Sprintf("Could not load channel %s from res map\n", m.UUID[5:]))
	}

	req <- m
	fmt.Printf("waiting for %s\n", m.UUID[:5])
	r := <-resChan.(chan *Message)
	if r.UUID != m.UUID {
		panic(fmt.Sprintf("Expected %s, got %s\n", m.UUID[:5], r.UUID[:5]))
	}
	res.Delete(r.UUID)
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

		// ZMQ send and receive
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

		// Handle the reply
		for _, r := range messages {
			resChan, ok := res.Load(r.UUID)
			if !ok {
				panic(fmt.Sprintf("Could not load channel %s from res map\n", r.UUID[5:]))
			}
			fmt.Printf("sending %s\n", r.UUID[:5])
			resChan.(chan *Message) <- r
		}
	}
}
