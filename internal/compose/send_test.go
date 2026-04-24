package compose

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSendServiceSuccessfulSend(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	calls := 0
	state := SendService{
		Now: func() time.Time { return now },
		Send: func(_ context.Context, raw []byte) error {
			calls++
			if string(raw) != "raw" {
				t.Fatalf("raw = %q", string(raw))
			}
			return nil
		},
	}.SendOrQueue(context.Background(), "acct-1", []byte("raw"))

	if calls != 1 || state.Status != SendStatusSent || state.SentAt == nil || !state.SentAt.Equal(now) {
		t.Fatalf("state = %#v calls=%d", state, calls)
	}
}

func TestSendServiceQueuesOnSendFailure(t *testing.T) {
	sendErr := errors.New("network down")
	queued := false
	state := SendService{
		Now: func() time.Time { return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC) },
		Send: func(context.Context, []byte) error {
			return sendErr
		},
		Queue: func(_ context.Context, accountID string, raw []byte, _ time.Time, errText string) error {
			queued = accountID == "acct-1" && string(raw) == "raw" && errText == sendErr.Error()
			return nil
		},
	}.SendOrQueue(context.Background(), "acct-1", []byte("raw"))

	if !queued || state.Status != SendStatusQueued || state.LastError != sendErr.Error() {
		t.Fatalf("state = %#v queued=%v", state, queued)
	}
}

func TestSendServiceFailsWhenQueueUnavailable(t *testing.T) {
	state := SendService{
		Send: func(context.Context, []byte) error {
			return errors.New("network down")
		},
	}.SendOrQueue(context.Background(), "acct-1", []byte("raw"))

	if state.Status != SendStatusFailed || state.LastError != "outbox queue unavailable" {
		t.Fatalf("state = %#v", state)
	}
}
