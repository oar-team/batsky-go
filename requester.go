package batskytime

// This file centralises the requests that has to be redirected to
// batkube.

type requestMessage struct {
	requestType string
	data        string
}

func request(req *requestMessage) {
}
