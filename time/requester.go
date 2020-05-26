package time

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	zmq "github.com/pebbe/zmq4"
)

// This code centralises the requests that have to be redirected to
// batkube.

type request struct {
	duration int64
	uuid     uuid.UUID
}

var req = make(chan *request)

// res is a map[uuid](chan int64)
//
// The sync map and UUID system is here to sync requests with replies.
// These uuid are not forwarded to the Broker.
//
// Some other technique have been tested to try to avoid the cost of
// hashing keys and storing channels for every single requests, but failed.
//
// The uuid system is very secure because it syncs requests and replies one by one
// without having to block any of the code to avoid sending to closed channels
// and whatnot.
var res = sync.Map{}

var responder *zmq.Socket

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

	var m request
	m.duration = d
	m.uuid = uuid.New()

	_, ok := res.Load(m.uuid)
	for ok {
		// This would be very, very unlucky. Supposedly it will never ever happen.
		fmt.Printf(fmt.Sprintf("Map entry already set for UUID %s. Generating a new one\n", m.uuid))
		m.uuid = uuid.New()
		_, ok = res.Load(m.uuid)
	}
	resChan := make(chan int64)
	res.Store(m.uuid, resChan)

	req <- &m
	return <-resChan
}

func run() {
	// Only one of these can be running at a time
	if running {
		panic("Two run() calls have been made simualtenously")
	}
	running = true

	sockAdress := "tcp://127.0.0.1:27000"
	fmt.Println("Creating new responder socket for time requests on", sockAdress)
	responder, _ = zmq.NewSocket(zmq.REP)
	responder.Connect(sockAdress)
	defer responder.Close()

	// This has to be a loop, otherwise not leaving any message behind
	// becomes very tricky.
	for {
		// One solution to the sync problem with batkube.
		// Batsim tells us when it's ready, so that we know when to
		// consume messages from the req channel
		fmt.Println("[batsky-go/time] Waiting for batkube ready signal")
		readyBytes, _ := responder.RecvBytes(0)

		ready := string(readyBytes)
		if ready != "ready" {
			panic(fmt.Sprintf("Failed handshake : Expected %s, got %s", "ready", ready))
		}

		// Using a range implies having to close req, which can't be done
		// in this situation.
		closeReq := false
		requests := make([]*request, 0)
		timerRequests := make([]int64, 0)
		for !closeReq {
			select {
			case m := <-req:
				requests = append(requests, m)
				if m.duration > 0 {
					timerRequests = append(timerRequests, m.duration)
				}
			default:
				//if len(requests) > 0 {
				//	closeReq = true
				//}
				closeReq = true
			}
		}
		// Other requests between now and when we receive the time but
		// we can't do much about them : nothing tells us wether the
		// scheduler will send other requests once we have consumed all
		// pending requests.

		// Probably there is something more efficient than json for this.
		msg, err := json.Marshal(timerRequests)
		if err != nil {
			panic("Error marshaling message:" + err.Error())
		}
		fmt.Printf("[batsky-go/time] Sending %v\n", timerRequests)
		_, err = responder.SendBytes(msg, 0)
		if err != nil {
			panic("Error sending message: " + err.Error())
		}

		b, err := responder.RecvBytes(0)
		if err != nil {
			panic("Error receiving message:" + err.Error())
		}
		now := int64(binary.LittleEndian.Uint64(b))
		fmt.Printf("[batsky-go/time] Got now : %d\n", now)
		// overflow
		if now < 0 {
			now = 1<<63 - 1 // math.MaxInt64
		}

		// Send the replies
		for _, m := range requests {
			resChan, ok := res.Load(m.uuid)
			if !ok {
				panic(fmt.Sprintf("Could not load channel %s from res map\n", m.uuid.String()[5:]))
			}
			resChan.(chan int64) <- now
		}

		_, err = responder.SendBytes([]byte("done"), 0)
		fmt.Println("[batsky-go/time] Done signal sent")
	}
}
