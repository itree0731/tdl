package recovery

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-faster/errors"
	"github.com/gotd/td/bin"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"go.uber.org/zap"

	"github.com/iyear/tdl/core/logctx"
)

type recovery struct {
	ctx        context.Context
	newBackoff func() backoff.BackOff
}

// New creates a recovery middleware. newBackoff is called once per invoke:
// a single shared backoff.BackOff would be mutated concurrently by parallel
// RPCs (it is not thread-safe), racing and cross-polluting retry state.
func New(ctx context.Context, newBackoff func() backoff.BackOff) telegram.Middleware {
	return &recovery{
		ctx:        ctx,
		newBackoff: newBackoff,
	}
}

func (r *recovery) Handle(next tg.Invoker) telegram.InvokeFunc {
	return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
		log := logctx.From(r.ctx)

		return backoff.RetryNotify(func() error {
			if err := next.Invoke(ctx, input, output); err != nil {
				if r.shouldRecover(ctx, err) {
					return errors.Wrap(err, "recover")
				}

				return backoff.Permanent(err)
			}

			return nil
		}, r.newBackoff(), func(err error, duration time.Duration) {
			log.Debug("Wait for connection recovery", zap.Error(err), zap.Duration("duration", duration))
		})
	}
}

func (r *recovery) shouldRecover(ctx context.Context, err error) bool {
	// context in recovery is used to stop recovery process by external os signal, otherwise we will wait till max retries when user press ctrl+c
	select {
	case <-r.ctx.Done():
		return false
	case <-ctx.Done():
		return false
	default:
	}

	// we try recover when encountered any error that is not telegram business error
	_, ok := tgerr.As(err)

	return !ok
}
