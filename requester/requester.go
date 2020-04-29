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
// REQUEST
// data for a time request is ""
// data for a timer request is the duration of the timer
//
// REPLY
// in both cases, data is the current time.

type Message struct {
	RequestType string
	Data        string
	UUID        string
}

var req = make(chan *Message)

// The sync map and UUID system is here to sync requests with replies.
// It allows the broker to reply whenever he wants by storing the ids.
//
// It is not necessary anymore since the only reply we have is the current time,
// for every request and every time.
// We only need two res channels : one for the current request being processed,
// one other is for the next request. That way we don't anwser to requests
// ahead of time.
var res = sync.Map{}

var reqSocket *zmq.Socket
var handshakeSocket *zmq.Socket

var running bool

/*
Returns the current time given by Batsim, in nanoseconds
*/
func RequestTime() int64 {
	m := &Message{
		RequestType: "time",
	}

	return sendAndConvert(m)
}

func RequestTimer(d int64) int64 {
	// d is a Duration, in nanoseconds
	// Reduce precision to milliseconds and convert to seconds
	f := float64(d/1e6) / 1e3
	m := &Message{
		RequestType: "timer",
		Data:        fmt.Sprintf("%f", f),
	}

	r := sendAndConvert(m)
	return r
}

// sends the messag and converts the result to nanoseconds
func sendAndConvert(m *Message) int64 {
	r := send(m)
	t, _ := strconv.ParseFloat(r.Data, 64) // in seconds
	return int64(t * 1e9)
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
		// This would be very, very unlucky. Supposedly it will never ever happen.
		fmt.Printf(fmt.Sprintf("Map entry already set for UUID %s. Generating a new one\n", m.UUID))
		m.UUID = uuid.New().String()
		_, ok = res.Load(m.UUID)
	}
	resChan := make(chan *Message)
	res.Store(m.UUID, resChan)

	req <- m
	r := <-resChan
	if r.UUID != m.UUID {
		panic(fmt.Sprintf("Expected %s, got %s\n", m.UUID[:5], r.UUID[:5]))
	}
	res.Delete(r.UUID)
	return r
}

func run() {
	// Only one of these can be running at a time
	running = true

	fmt.Println("Creating new handshake socket for time requests")
	handshakeSocket, _ = zmq.NewSocket(zmq.REP)
	handshakeSocket.Connect("tcp://127.0.0.1:27001")

	fmt.Println("Creating new request socket for time requests")
	reqSocket, _ = zmq.NewSocket(zmq.REQ)
	reqSocket.Connect("tcp://127.0.0.1:27000")

	// This has to be a loop, otherwise not leaving any message behind
	// becomes very tricky.
	for {
		// One solution to the sync problem with batkube.
		// Requests are sent only when the broker makes us know it's ready.
		//
		// One other solution would be to keep only one socket and
		// inverse the roles (the broker is the requester). This would
		// result in a few more exchanges though to comply with the zmq
		// protocol. Having two sockets allows to send two subsequent
		// messages.
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
		messages := make([]*Message, 0)
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

		// ZMQ send and receive
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
			resChan.(chan *Message) <- r
		}
	}
}
