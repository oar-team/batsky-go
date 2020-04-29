package time

import (
	"fmt"
	"sync"
	"testing"
)

func TestMultipleRoutines(t *testing.T) {
	fmt.Println("\nA few routines")
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, id int) {
			defer wg.Done()
			fmt.Println(Now())
		}(&wg, i)
	}
	wg.Wait()
}

func TestHello(t *testing.T) {
	fmt.Println("\nSimple for loop")
	for i := 0; i < 4; i++ {
		fmt.Println(Now())
	}
}

func TestMultipleRoutinesDelayed(t *testing.T) {
	fmt.Println("\nA few routines delayed")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, id int) {
			defer wg.Done()
			fmt.Println(Now())
		}(&wg, i)
		//	time.Sleep(300 * time.Millisecond)
	}
	wg.Wait()
}

func TestMoreRoutines(t *testing.T) {
	fmt.Println("\nA lot of routines")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, id int) {
			defer wg.Done()
			fmt.Println(Now())
		}(&wg, i)
	}
	wg.Wait()
}

func TestEpoch(t *testing.T) {
	fmt.Println("\nTest unix epoch")
	now := Now()
	unix := now.Unix()
	nanos := now.UnixNano()
	fmt.Printf("now %v\nunix %d\nnanos %d\n", now, unix, nanos)
}
