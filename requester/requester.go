package requester

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	zmq "github.com/pebbe/zmq4"
)

// This code centralises the requests that have to be redirected to
// batkube.

var req = sync.Map{}
var res = sync.Map{}
var requestId = uint32(0)

var reqSocket *zmq.Socket
var handshakeSocket *zmq.Socket

var running bool

/*
Returns the current time given by Batsim, in nanoseconds.
If d is > 0, it tells batkube the scheduler requested for a timer.
If now is the current time, the timer is supposed to fire at
now + d.
This will send a CALL_ME_LATER event to Batsim with timestamp now + d.
*/
func RequestTime(d int64) int64 {
	// Could it be that two tests happen at the exact same time thus
	// calling run() twice?
	// TODO secure run() call?
	if !running {
		go run()
	}

	currRequestId := requestId
	resChan, _ := res.LoadOrStore(currRequestId, make(chan int64))
	reqChan, _ := req.LoadOrStore(currRequestId, make(chan int64))

	reqChan.(chan int64) <- d
	now := <-resChan.(chan int64)
	return now

}

func run() {
	// Only one of these can be running at a time
	if running {
		panic("Two run() calls have been made simualtenously")
	}
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

		resChan, _ := res.LoadOrStore(requestId, make(chan int64))
		reqChan, _ := req.LoadOrStore(requestId, make(chan int64))
		// The rest of the loop makes the assumption that every request from now on is made
		// for requestId which may not be true.
		// TODO : verify that
		requests := make([]int64, 0)
		// Using a range implies having to close req, which is prone to
		// panics in this situation
		closeReq := false
		reqNumber := 0
		currRequestId := requestId
		emptyReq := true
		for !closeReq {
			select {
			case d := <-reqChan.(chan int64):
				reqNumber++
				emptyReq = false
				if d > 0 {
					requests = append(requests, d)
				}
			default:
				// this assumes there will be other calls. Can potentially deadlock
				if reqNumber > 0 {
					if currRequestId == requestId {
						requestId++
					} else {
						// we make another round to make sure
						// req is empty, but maybe it's not
						// sufficient
						closeReq = true
						time.Sleep(10 * time.Millisecond)
					}
				} else if !emptyReq {
					emptyReq = true
					time.Sleep(10 * time.Millisecond)
				} else {
					closeReq = true
				}
			}
		}

		// ZMQ send and receive
		// Probably there is something more efficient than json for this.
		msg, err := json.Marshal(requests)
		if err != nil {
			panic(fmt.Sprintf("Error marshaling message %v:", requests) + err.Error())
		}
		_, err = reqSocket.SendBytes(msg, 0)
		if err != nil {
			panic("Error sending message: " + err.Error())
		}

		b, err := reqSocket.RecvBytes(0)
		if err != nil {
			panic("Error receiving message:" + err.Error())
		}
		now := int64(binary.LittleEndian.Uint64(b))
		//fmt.Printf("request id %d; nb %d; now %f\n", currRequestId, reqNumber, float64(now/1e6)/1e3)

		// Send the replies
		for i := 0; i < reqNumber; i++ {
			resChan.(chan int64) <- now
		}
		//TODO : delete those channels?
	}
}
