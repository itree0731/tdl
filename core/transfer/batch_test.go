package transfer

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestMiddleItemFailsAndLastStillRuns(t *testing.T) {
	failed := errors.New("second item failed")
	index := 0
	var mu sync.Mutex
	runItems := map[int]bool{}
	err := Run(context.Background(), 2,
		func(context.Context) bool { return index < 3 },
		func() int { index++; return index },
		func() error { return nil },
		func(_ context.Context, item int) error {
			mu.Lock()
			runItems[item] = true
			mu.Unlock()
			if item == 2 {
				return failed
			}
			return nil
		})
	var batch *BatchError
	if !errors.As(err, &batch) || !errors.Is(err, failed) || len(batch.Errors) != 1 || !runItems[3] {
		t.Fatalf("err=%v ran=%v", err, runItems)
	}
}
