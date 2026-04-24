package compose

import (
	"context"
	"time"
)

type SendFunc func(context.Context, []byte) error
type QueueFunc func(context.Context, string, []byte, time.Time, string) error

type SendService struct {
	Send  SendFunc
	Queue QueueFunc
	Now   func() time.Time
}

func (s SendService) SendOrQueue(ctx context.Context, accountID string, raw []byte) SendState {
	now := func() time.Time { return time.Now().UTC() }
	if s.Now != nil {
		now = s.Now
	}
	if s.Send == nil {
		return SendState{Status: SendStatusFailed, LastError: "send unavailable"}
	}
	if err := s.Send(ctx, raw); err != nil {
		state := SendState{Status: SendStatusQueued, LastError: err.Error()}
		if s.Queue != nil {
			if queueErr := s.Queue(ctx, accountID, raw, now(), err.Error()); queueErr != nil {
				state.Status = SendStatusFailed
				state.LastError = queueErr.Error()
			}
		}
		return state
	}
	sentAt := now()
	return SendState{Status: SendStatusSent, SentAt: &sentAt}
}
