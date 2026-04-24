package gmail

import (
	"context"
	"errors"
	"time"
)

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Sleep       func(context.Context, time.Duration) error
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		Sleep:       sleepContext,
	}
}

func (c *Client) SetRetryPolicy(policy RetryPolicy) {
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = 1
	}
	if policy.Sleep == nil {
		policy.Sleep = sleepContext
	}
	c.retry = policy
}

func (c *Client) withRetry(ctx context.Context, operation string, call func() error) error {
	policy := c.retry
	if policy.MaxAttempts <= 0 {
		policy = DefaultRetryPolicy()
	}
	var err error
	for attempt := 0; attempt < policy.MaxAttempts; attempt++ {
		err = normalizeError(operation, call())
		if err == nil {
			return nil
		}
		if attempt == policy.MaxAttempts-1 || !retryableError(err) {
			return err
		}
		if sleepErr := policy.Sleep(ctx, retryDelay(policy, attempt)); sleepErr != nil {
			return normalizeError(operation, sleepErr)
		}
	}
	return err
}

func retryableError(err error) bool {
	return errors.Is(err, ErrRateLimited) || errors.Is(err, ErrUnavailable)
}

func retryDelay(policy RetryPolicy, attempt int) time.Duration {
	delay := policy.BaseDelay
	if delay <= 0 {
		delay = 100 * time.Millisecond
	}
	for i := 0; i < attempt; i++ {
		delay *= 2
	}
	if policy.MaxDelay > 0 && delay > policy.MaxDelay {
		return policy.MaxDelay
	}
	return delay
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
