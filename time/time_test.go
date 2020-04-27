package time

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestHello(t *testing.T) {
	fmt.Println("simple for loop")
	for i := 0; i < 4; i++ {
		fmt.Println(Now())
	}
}

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

func TestMultipleRoutinesDelayed(t *testing.T) {
	fmt.Println("\nA few routines delayed")
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, id int) {
			defer wg.Done()
			fmt.Println(Now())
		}(&wg, i)
		time.Sleep(400 * time.Millisecond)
	}
	wg.Wait()
}
