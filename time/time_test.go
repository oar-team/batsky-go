package time

import (
	"sync"
	"testing"
)

func TestSimpleForloop(t *testing.T) {
	for i := 0; i < 4; i++ {
		t.Log(Now())
	}
}

func TestMultipleRoutines(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, id int) {
			defer wg.Done()
			t.Log(Now())
		}(&wg, i)
	}
	wg.Wait()
}

func TestMoreRoutines(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, id int) {
			defer wg.Done()
			t.Log(Now())
		}(&wg, i)
	}
	wg.Wait()
}

func TestEpoch(t *testing.T) {
	now := Now()
	unix := now.Unix()
	nanos := now.UnixNano()
	t.Logf("now %v\nunix %d\nnanos %d\n", now, unix, nanos)
}
