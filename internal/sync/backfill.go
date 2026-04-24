package sync

import (
	"context"
	"fmt"
	"sort"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/gmail"
	"github.com/jmoiron/sqlx"
)

const (
	metadataBatchSize = 100
	listPageSize      = 500
)

var MetadataHeaders = []string{"From", "To", "Cc", "Bcc", "Subject", "Date"}

type Backfiller struct {
	config    config.Config
	gmail     gmail.MessageReader
	labels    cache.LabelRepository
	threads   cache.ThreadRepository
	messages  cache.MessageRepository
	msgLabels cache.MessageLabelRepository
	attach    cache.AttachmentRepository
	evaluator PolicyEvaluator
}

type BackfillResult struct {
	Labels        []BackfilledLabel
	Messages      []gmail.Message
	BodyQueue     []BodyFetchRequest
	ExcludedCount int
}

type BackfilledLabel struct {
	LabelID      string
	Depth        config.CacheDepth
	Listed       int
	RetentionDay *int
}

type BodyFetchRequest struct {
	MessageID string
	ThreadID  string
	Depth     config.CacheDepth
}

func NewBackfiller(db *sqlx.DB, cfg config.Config, client gmail.MessageReader) Backfiller {
	return Backfiller{
		config:    cfg,
		gmail:     client,
		labels:    cache.NewLabelRepository(db),
		threads:   cache.NewThreadRepository(db),
		messages:  cache.NewMessageRepository(db),
		msgLabels: cache.NewMessageLabelRepository(db),
		attach:    cache.NewAttachmentRepository(db),
		evaluator: NewPolicyEvaluator(db, cfg),
	}
}

func (b Backfiller) Backfill(ctx context.Context, account cache.Account) (BackfillResult, error) {
	if b.gmail == nil {
		return BackfillResult{}, fmt.Errorf("gmail client is required")
	}

	labels, err := b.labels.List(ctx, account.ID)
	if err != nil {
		return BackfillResult{}, err
	}

	return b.backfillLabels(ctx, account, labels)
}

func (b Backfiller) BackfillLabel(ctx context.Context, account cache.Account, labelID string) (BackfillResult, error) {
	if b.gmail == nil {
		return BackfillResult{}, fmt.Errorf("gmail client is required")
	}

	labels, err := b.labels.List(ctx, account.ID)
	if err != nil {
		return BackfillResult{}, err
	}
	for _, label := range labels {
		if label.ID == labelID {
			return b.backfillLabels(ctx, account, []cache.Label{label})
		}
	}
	return BackfillResult{}, fmt.Errorf("label %q is not cached", labelID)
}

func (b Backfiller) backfillLabels(ctx context.Context, account cache.Account, labels []cache.Label) (BackfillResult, error) {
	result := BackfillResult{}
	refsByID := map[string]gmail.MessageRef{}
	labelsByMessage := map[string][]string{}

	for _, label := range labels {
		policy, err := b.evaluator.EffectiveForLabel(ctx, account.ID, account.Email, label.ID)
		if err != nil {
			return BackfillResult{}, err
		}
		if !policy.Include || policy.Depth == config.CacheDepthNone {
			continue
		}

		refs, err := b.listLabelMessages(ctx, label.ID, policy.RetentionDays)
		if err != nil {
			return BackfillResult{}, err
		}
		result.Labels = append(result.Labels, BackfilledLabel{
			LabelID:      label.ID,
			Depth:        policy.Depth,
			Listed:       len(refs),
			RetentionDay: cloneInt(policy.RetentionDays),
		})
		for _, ref := range refs {
			if ref.ID == "" {
				continue
			}
			refsByID[ref.ID] = ref
			labelsByMessage[ref.ID] = appendUnique(labelsByMessage[ref.ID], label.ID)
		}
	}

	ids := orderedMessageIDs(refsByID)
	for start := 0; start < len(ids); start += metadataBatchSize {
		end := start + metadataBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		messages, err := b.gmail.BatchGetMessageMetadata(ctx, ids[start:end], MetadataHeaders...)
		if err != nil {
			return BackfillResult{}, err
		}
		for _, message := range messages {
			if message.ID == "" {
				continue
			}
			ref := refsByID[message.ID]
			message.ThreadID = firstNonEmpty(message.ThreadID, ref.ThreadID)
			labelIDs := mergeLabels(labelsByMessage[message.ID], message.LabelIDs)
			decision, err := b.evaluator.Evaluate(ctx, MessageContext{
				AccountID:    account.ID,
				AccountEmail: account.Email,
				From:         headerValue(message.Headers, "From"),
				LabelIDs:     labelIDs,
			})
			if err != nil {
				return BackfillResult{}, err
			}
			if decision.Excluded {
				result.ExcludedCount++
				continue
			}
			if err := b.persistMetadata(ctx, account.ID, message, labelIDs); err != nil {
				return BackfillResult{}, err
			}
			result.Messages = append(result.Messages, message)
			if decision.Depth == config.CacheDepthBody || decision.Depth == config.CacheDepthFull {
				result.BodyQueue = append(result.BodyQueue, BodyFetchRequest{
					MessageID: message.ID,
					ThreadID:  firstNonEmpty(message.ThreadID, ref.ThreadID),
					Depth:     decision.Depth,
				})
			}
		}
	}

	return result, nil
}

func (b Backfiller) listLabelMessages(ctx context.Context, labelID string, retentionDays *int) ([]gmail.MessageRef, error) {
	limit := b.config.Sync.BackfillLimitPerLabel
	if limit <= 0 {
		limit = config.Default().Sync.BackfillLimitPerLabel
	}

	refs := []gmail.MessageRef{}
	pageToken := ""
	for len(refs) < limit {
		remaining := limit - len(refs)
		maxResults := listPageSize
		if remaining < maxResults {
			maxResults = remaining
		}

		page, err := b.gmail.ListMessages(ctx, gmail.ListMessagesOptions{
			LabelIDs:   []string{labelID},
			Query:      retentionQuery(retentionDays),
			PageToken:  pageToken,
			MaxResults: int64(maxResults),
		})
		if err != nil {
			return nil, err
		}
		refs = append(refs, page.Messages...)
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	if len(refs) > limit {
		refs = refs[:limit]
	}
	return refs, nil
}

func retentionQuery(retentionDays *int) string {
	if retentionDays == nil || *retentionDays <= 0 {
		return ""
	}
	return fmt.Sprintf("newer_than:%dd", *retentionDays)
}

func orderedMessageIDs(refs map[string]gmail.MessageRef) []string {
	ids := make([]string, 0, len(refs))
	for id := range refs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func mergeLabels(a []string, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	for _, value := range a {
		out = appendUnique(out, value)
	}
	for _, value := range b {
		out = appendUnique(out, value)
	}
	return out
}

func headerValue(headers []gmail.MessageHeader, name string) string {
	for _, header := range headers {
		if equalHeaderName(header.Name, name) {
			return header.Value
		}
	}
	return ""
}

func equalHeaderName(a string, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca := a[i]
		cb := b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
