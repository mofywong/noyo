package workorder

import (
	"context"
	"time"
)

type DeliveryAdapter interface {
	Deliver(context.Context, OutboxMessage) error
}
type OutboxWorker struct {
	Store       *OutboxStore
	Adapter     DeliveryAdapter
	MaxAttempts int
}

func (w *OutboxWorker) RunOnce(ctx context.Context) error {
	if w.Store == nil || w.Adapter == nil {
		return NewError(CodeValidationFailed, "outbox worker is not configured")
	}
	attempts := w.MaxAttempts
	if attempts <= 0 {
		attempts = 5
	}
	messages, token := w.Store.Claim(nowUTC(), 30, 50)
	for _, message := range messages {
		if err := w.Adapter.Deliver(ctx, message); err != nil {
			if failErr := w.Store.Fail(message.ID, token, err, attempts); failErr != nil {
				return failErr
			}
			continue
		}
		if err := w.Store.Ack(message.ID, token); err != nil {
			return err
		}
	}
	return nil
}
func nowUTC() (t time.Time) { return time.Now().UTC() }
