package internal

import (
	"sync"
)

// WaitGroupDone returns a channel that will be closed when the wait group is done.
func WaitGroupDone(wg *sync.WaitGroup) <-chan byte {
	ch := make(chan byte)
	go func() {
		defer close(ch)
		wg.Wait()
	}()
	return ch
}
