package sync

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/gmail"
)

func (b Backfiller) persistMetadata(ctx context.Context, accountID string, message gmail.Message, labelIDs []string) error {
	messageDate := parsedMessageDate(message.Headers)
	lastMessageDate := timePtr(message.InternalDate)
	if lastMessageDate == nil {
		lastMessageDate = messageDate
	}
	threadID := message.ThreadID
	if threadID == "" {
		threadID = message.ID
	}

	if err := b.upsertThreadMetadata(ctx, cache.Thread{
		AccountID:       accountID,
		ID:              threadID,
		Snippet:         message.Snippet,
		HistoryID:       message.HistoryID,
		LastMessageDate: lastMessageDate,
	}); err != nil {
		return err
	}

	internalDate := timePtr(message.InternalDate)
	if err := b.messages.Upsert(ctx, cache.Message{
		AccountID:    accountID,
		ID:           message.ID,
		ThreadID:     threadID,
		FromAddr:     headerValue(message.Headers, "From"),
		ToAddrs:      parsedAddressList(headerValue(message.Headers, "To")),
		CcAddrs:      parsedAddressList(headerValue(message.Headers, "Cc")),
		BccAddrs:     parsedAddressList(headerValue(message.Headers, "Bcc")),
		Subject:      headerValue(message.Headers, "Subject"),
		Date:         messageDate,
		Snippet:      message.Snippet,
		SizeBytes:    message.SizeEstimate,
		RawHeaders:   cacheHeaders(message.Headers),
		InternalDate: internalDate,
		FetchedFull:  false,
	}); err != nil {
		return err
	}

	return b.msgLabels.ReplaceForMessage(ctx, accountID, message.ID, labelIDs)
}

func (b Backfiller) upsertThreadMetadata(ctx context.Context, thread cache.Thread) error {
	existing, err := b.threads.Get(ctx, thread.AccountID, thread.ID)
	if err == nil && existing.LastMessageDate != nil && thread.LastMessageDate != nil && existing.LastMessageDate.After(*thread.LastMessageDate) {
		thread.LastMessageDate = existing.LastMessageDate
	}
	if err != nil && !errors.Is(err, cache.ErrThreadNotFound) {
		return err
	}
	return b.threads.Upsert(ctx, thread)
}

func parsedMessageDate(headers []gmail.MessageHeader) *time.Time {
	value := headerValue(headers, "Date")
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := mail.ParseDate(value)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}

func parsedAddressList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	addresses, err := mail.ParseAddressList(value)
	if err != nil {
		return []string{value}
	}
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		out = append(out, address.Address)
	}
	return out
}

func cacheHeaders(headers []gmail.MessageHeader) []cache.Header {
	out := make([]cache.Header, 0, len(headers))
	for _, header := range headers {
		out = append(out, cache.Header{Name: header.Name, Value: header.Value})
	}
	return out
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	out := value.UTC()
	return &out
}
