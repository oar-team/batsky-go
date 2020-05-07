# batsky-go

Modified Go time library for Kubernetes schedulers running on a Batsim
simulation through Batkube.

It redirects all calls trying to get machine time to the Batkube message
broker.

## Usage
Simply replace the "time" import to this repo's time
("github.com/oar-team/batsky-go/time") in any dependancy where you want to
redirect time calls.

For already established projects that you would like to fork in order to hijack
the time, here are a few steps you can follow for projects using vgo (go.mod
and gop.sum) :

* Fork the repo, clone it, `cd` into it.
* If you wish to change its name, you need to change the module's name in
    go.mod but also in all internal imports to this module's packages.
* Change "time" imports to this repository's time.
* `go mod tidy`
* `go get -u github.com/oar-team/batsky-go@master` to get the latest
    commit (as this is yet a proof of concept, commits are not consistently
    tagged).

These forks were created with this intent and use batsky-go, you may change imports to these ones :
* [batsky-apimachinery](https://github.com/oar-team/batsky-apimachinery)
* [batsky-client-go](https://github.com/oar-team/batsky-client-go)

Side notes : 
- For now, relevant changes are on "batsky" branches, not master, so you have to `go get -u github.com/oar-team/package@batsky` to get the dependencies.
- Replacing the official dependencies for the fork in the go.mod appears to be not possible at the moment (see [#26904](https://github.com/golang/go/issues/26904) issue on Go's repos) or bugged (the replace ignores the pinned version I want to give it). Imports have to be manually replaced.


## Principles
All calls get piled up in requester.go and sent to Batkube whenever the broker
says it is ready. The response, which is the current simulation time, is then
returned back to the original callers.

Timer requests break down to regular time requests, except they include a
non-zero duration parameter. Those requests are forwarded to Batsim in the form
of a CALL_ME_LATER event which will be anwsered with a REQUESTED_CALL when the
timer is supposed to fire.

This is essential to the scheduler, as otherwise it would have to wait for the
next simulation event to get an update on the time and wake the timers up,
which would maybe not happen for a while (in sumulation time).

### zmq exchanges breakdown
One exchange starts of with a "handshake" initiated by the broker (Batkube),
which tells the requester (in the time package) it is ready to process the time
requests. From there, the requester consumes all pending requests and forwards
them to the broker.

This allows requests from the scheduler to be taken into account while the
requester is waiting for the borker to be able to receive the message. The
pending requests would otherwise be sent on the next exchange, introducing a
delay which could be harmful to the scheduler and the simulation in general.

Whether the requester has to wait for any time requests to come up to even send
a message to Batsim is a design decision we have to make.

Here is a diagram to better illustrate those exchanges.

![requester - broker exchanges](imgs/requester-broker.png)

### Requester inner mechanics
`requester.go` is composed of two main functions :
* The main loop `run` handles all requests, forwards them to Batkube and
sends over the current simulation time to all callers of `RequestTime`. There
can only be one running at a time.
* `RequestTime` sends the requests to the main loop. There are multiple
instances of these at a time, and calls to `RequestTime` are unpredictable.

Here is the main loop pseudo-algorithm : 
![main loop](imgs/alg-req-loop.png)

And here is how `RequestTime` unfolds :
![request time](imgs/alg-now.png)

## TODO
- helper script to change imports with doc

