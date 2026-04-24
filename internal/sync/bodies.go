package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/gmail"
	"github.com/bkarlovitz/gandt/internal/render"
)

type BodyFetchResult struct {
	Requested int
	Fetched   int
	Cached    int
}

func (b Backfiller) FetchBodies(ctx context.Context, account cache.Account, queue []BodyFetchRequest) (BodyFetchResult, error) {
	if b.gmail == nil {
		return BodyFetchResult{}, fmt.Errorf("gmail client is required")
	}

	result := BodyFetchResult{Requested: len(queue)}
	for _, request := range queue {
		message, err := b.gmail.GetMessageFull(ctx, request.MessageID)
		if err != nil {
			return result, err
		}
		message.ThreadID = firstNonEmpty(message.ThreadID, request.ThreadID)
		cached, err := b.PersistFullMessage(ctx, account, message)
		if err != nil {
			return result, err
		}
		result.Fetched++
		if cached {
			result.Cached++
		}
	}
	return result, nil
}

func (b Backfiller) PersistFullMessage(ctx context.Context, account cache.Account, message gmail.Message) (bool, error) {
	threadID := firstNonEmpty(message.ThreadID, message.ID)
	message.ThreadID = threadID
	labelIDs := mergeLabels(nil, message.LabelIDs)

	decision, err := b.evaluator.Evaluate(ctx, MessageContext{
		AccountID:    account.ID,
		AccountEmail: account.Email,
		From:         headerValue(message.Headers, "From"),
		LabelIDs:     labelIDs,
	})
	if err != nil {
		return false, err
	}
	if decision.Excluded || !decision.Persist {
		return false, nil
	}

	extracted, err := gmail.ExtractBody(message, gmail.BodyExtractionOptions{KeepHTML: decision.Depth == config.CacheDepthFull})
	if err != nil {
		return false, err
	}
	bodyPlain, err := persistedPlainBody(extracted)
	if err != nil {
		return false, err
	}
	bodyHTML := persistedHTMLBody(extracted, decision.Depth)

	messageDate := parsedMessageDate(message.Headers)
	lastMessageDate := timePtr(message.InternalDate)
	if lastMessageDate == nil {
		lastMessageDate = messageDate
	}
	if err := b.upsertThreadMetadata(ctx, cache.Thread{
		AccountID:       account.ID,
		ID:              threadID,
		Snippet:         message.Snippet,
		HistoryID:       message.HistoryID,
		LastMessageDate: lastMessageDate,
	}); err != nil {
		return false, err
	}

	var cachedAt *time.Time
	if bodyPlain != nil || bodyHTML != nil {
		now := time.Now().UTC()
		cachedAt = &now
	}
	if err := b.messages.Upsert(ctx, cache.Message{
		AccountID:    account.ID,
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
		BodyPlain:    bodyPlain,
		BodyHTML:     bodyHTML,
		RawHeaders:   cacheHeaders(message.Headers),
		InternalDate: timePtr(message.InternalDate),
		FetchedFull:  decision.Depth == config.CacheDepthFull && (bodyPlain != nil || bodyHTML != nil),
		CachedAt:     cachedAt,
	}); err != nil {
		return false, err
	}
	if err := b.msgLabels.ReplaceForMessage(ctx, account.ID, message.ID, labelIDs); err != nil {
		return false, err
	}
	if decision.Depth == config.CacheDepthFull {
		if err := b.persistAttachmentMetadata(ctx, account.ID, message.ID, extracted.Attachments, decision); err != nil {
			return false, err
		}
	}

	return cachedAt != nil, nil
}

func persistedPlainBody(extracted gmail.ExtractedBody) (*string, error) {
	if extracted.Plain != nil {
		return extracted.Plain, nil
	}
	if extracted.FallbackHTML == nil {
		return nil, nil
	}
	text, err := render.HTMLToText(*extracted.FallbackHTML, render.HTMLRenderOptions{URLFootnotes: true})
	if err != nil {
		return nil, err
	}
	return &text, nil
}

func persistedHTMLBody(extracted gmail.ExtractedBody, depth config.CacheDepth) *string {
	if depth != config.CacheDepthFull {
		return nil
	}
	return extracted.HTML
}

func (b Backfiller) persistAttachmentMetadata(ctx context.Context, accountID string, messageID string, attachments []gmail.MIMEAttachment, decision CacheDecision) error {
	for _, attachment := range attachments {
		if !shouldPersistAttachment(attachment, decision) {
			continue
		}
		partID := firstNonEmpty(attachment.PartID, attachment.Filename, attachment.AttachmentID)
		if partID == "" {
			continue
		}
		if err := b.attach.Upsert(ctx, cache.Attachment{
			AccountID:    accountID,
			MessageID:    messageID,
			PartID:       partID,
			Filename:     attachment.Filename,
			MimeType:     attachment.MimeType,
			SizeBytes:    attachment.Size,
			AttachmentID: attachment.AttachmentID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func shouldPersistAttachment(attachment gmail.MIMEAttachment, decision CacheDecision) bool {
	switch decision.AttachmentRule {
	case config.AttachmentRuleAll:
		return true
	case config.AttachmentRuleUnderSize:
		if decision.AttachmentMaxMB == nil {
			return true
		}
		return attachment.Size <= *decision.AttachmentMaxMB*1024*1024
	default:
		return false
	}
}
