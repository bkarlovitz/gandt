package sync

import (
	"context"
	"time"

	"github.com/bkarlovitz/gandt/internal/cache"
)

type OutboxSender interface {
	SendMessage(context.Context, []byte) error
}

type OutboxRetryResult struct {
	Sent    int
	Failed  int
	Skipped int
}

type OutboxRetryer struct {
	Repository  cache.OutboxRepository
	Sender      OutboxSender
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	MaxAttempts int
	Now         func() time.Time
}

func (r OutboxRetryer) Retry(ctx context.Context, accountID string) (OutboxRetryResult, error) {
	now := func() time.Time { return time.Now().UTC() }
	if r.Now != nil {
		now = r.Now
	}
	base := r.BaseBackoff
	if base <= 0 {
		base = time.Second
	}
	maxBackoff := r.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = time.Minute
	}
	maxAttempts := r.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	pending, err := r.Repository.Pending(ctx, accountID, 0)
	if err != nil {
		return OutboxRetryResult{}, err
	}
	var result OutboxRetryResult
	for _, message := range pending {
		if !outboxRetryDue(message, now(), base, maxBackoff) {
			result.Skipped++
			continue
		}
		if r.Sender == nil {
			result.Failed++
			if err := r.Repository.MarkFailed(ctx, message.ID, "outbox sender unavailable"); err != nil {
				return result, err
			}
			continue
		}
		if err := r.Sender.SendMessage(ctx, message.RawRFC822); err != nil {
			if message.Attempts+1 >= maxAttempts {
				result.Failed++
				if markErr := r.Repository.MarkFailed(ctx, message.ID, err.Error()); markErr != nil {
					return result, markErr
				}
				continue
			}
			if markErr := r.Repository.MarkRetry(ctx, message.ID, err.Error()); markErr != nil {
				return result, markErr
			}
			continue
		}
		result.Sent++
		if err := r.Repository.MarkSent(ctx, message.ID); err != nil {
			return result, err
		}
	}
	return result, nil
}

func outboxRetryDue(message cache.OutboxMessage, now time.Time, base time.Duration, maxBackoff time.Duration) bool {
	delay := base
	for i := 0; i < message.Attempts; i++ {
		delay *= 2
		if delay >= maxBackoff {
			delay = maxBackoff
			break
		}
	}
	return !now.Before(message.QueuedAt.Add(delay))
}
