package gatewaysvc

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlightGroupDeduplicatesConcurrentWork(t *testing.T) {
	group := newFlightGroup()
	var calls int32
	var wg sync.WaitGroup
	results := make(chan string, 8)

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, _, err := group.Do("cache-key", func() (any, error) {
				atomic.AddInt32(&calls, 1)
				time.Sleep(20 * time.Millisecond)
				return "fresh", nil
			})
			if err != nil {
				t.Errorf("Do: %v", err)
				return
			}
			results <- v.(string)
		}()
	}
	wg.Wait()
	close(results)

	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	for got := range results {
		if got != "fresh" {
			t.Fatalf("result = %q", got)
		}
	}
}
