package transfer

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestFailureDoesNotCancelRemainingItems(t *testing.T) {
	sentinel := errors.New("second file failed")
	i := 0
	var ran []int
	sealed := false
	err := Run(context.Background(), 1, func(context.Context) bool { return i < 3 }, func() int { i++; return i }, func() error { return nil }, func(_ context.Context, n int) error {
		ran = append(ran, n)
		if n == 2 {
			return sentinel
		}
		return nil
	}, func() { sealed = true })
	var batch *BatchError
	if !errors.As(err, &batch) || !errors.Is(err, sentinel) || !reflect.DeepEqual(ran, []int{1, 2, 3}) || !sealed {
		t.Fatalf("err=%v ran=%v sealed=%v", err, ran, sealed)
	}
}
func TestCancellationAndIterationErrorsArePreserved(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	i := 0
	err := Run(ctx, 1, func(c context.Context) bool { return c.Err() == nil && i < 10 }, func() int { i++; return i }, func() error { return ctx.Err() }, func(c context.Context, n int) error { cancel(); return c.Err() }, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
	sentinel := errors.New("iterator failed")
	err = Run(context.Background(), 1, func(context.Context) bool { return false }, func() int { return 0 }, func() error { return sentinel }, func(context.Context, int) error { return nil }, nil)
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}
