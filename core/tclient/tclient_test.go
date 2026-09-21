package tclient

import (
	"context"
	"errors"
	"testing"
)

func TestRunnerCannotNormalizeCallbackFailureToSuccess(t *testing.T) {
	swallowed := func(ctx context.Context, f func(context.Context) error) error { _ = f(ctx); return nil }
	failure := errors.Join(errors.New("batch failure"), context.Canceled)
	if err := runPreservingError(context.Background(), swallowed, func(context.Context) error { return failure }); !errors.Is(err, failure) || !errors.Is(err, context.Canceled) {
		t.Fatalf("lost callback error: %v", err)
	}
	if err := runPreservingError(context.Background(), swallowed, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
}
func TestCanceledParentWithoutCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runPreservingError(ctx, func(context.Context, func(context.Context) error) error { return nil }, func(context.Context) error { t.Fatal("unexpected callback"); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
