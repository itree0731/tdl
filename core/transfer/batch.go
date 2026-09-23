// Package transfer runs transfer batches without hiding individual failures.
package transfer

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// BatchError makes a partially failed batch visible to CLI callers while
// retaining the individual errors for errors.Is and errors.As.
type BatchError struct{ Errors []error }

func (e *BatchError) Error() string {
	return fmt.Sprintf("%d transfer error(s): %v", len(e.Errors), errors.Join(e.Errors...))
}
func (e *BatchError) Unwrap() []error { return e.Errors }

func Run[T any](ctx context.Context, limit int, next func(context.Context) bool, value func() T, iterErr func() error, run func(context.Context, T) error) error {
	if limit < 1 {
		return fmt.Errorf("transfer concurrency must be positive")
	}
	group, workCtx := errgroup.WithContext(ctx)
	group.SetLimit(limit)
	var mu sync.Mutex
	var failures []error
	record := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		failures = append(failures, err)
		mu.Unlock()
	}
	for next(workCtx) {
		item := value()
		group.Go(func() error {
			err := run(workCtx, item)
			record(err)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) && workCtx.Err() != nil {
				return err
			}
			return nil
		})
	}
	record(iterErr())
	_ = group.Wait()
	if len(failures) > 0 {
		return &BatchError{Errors: failures}
	}
	return ctx.Err()
}
