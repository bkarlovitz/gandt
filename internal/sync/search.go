package sync

import (
	"context"
	"fmt"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/gmail"
)

const defaultSearchPageSize int64 = 50

type OnlineSearcher struct {
	gmail gmail.MessageReader
}

type OnlineSearchRequest struct {
	Query      string
	MaxResults int
	PageSize   int64
}

type OnlineSearchResult struct {
	Messages           []gmail.Message
	ResultSizeEstimate int
}

type SearchPersistenceResult struct {
	MetadataCached int
	BodyQueue      []BodyFetchRequest
	ExcludedCount  int
}

func NewOnlineSearcher(reader gmail.MessageReader) OnlineSearcher {
	return OnlineSearcher{gmail: reader}
}

func (s OnlineSearcher) Search(ctx context.Context, request OnlineSearchRequest) (OnlineSearchResult, error) {
	if s.gmail == nil {
		return OnlineSearchResult{}, fmt.Errorf("gmail search unavailable")
	}
	limit := request.MaxResults
	if limit <= 0 {
		limit = 100
	}
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = defaultSearchPageSize
	}
	result := OnlineSearchResult{}
	pageToken := ""
	for len(result.Messages) < limit {
		if err := ctx.Err(); err != nil {
			return OnlineSearchResult{}, err
		}
		remaining := int64(limit - len(result.Messages))
		maxResults := minInt64(pageSize, remaining)
		page, err := s.gmail.ListMessages(ctx, gmail.ListMessagesOptions{
			Query:      request.Query,
			PageToken:  pageToken,
			MaxResults: maxResults,
		})
		if err != nil {
			return OnlineSearchResult{}, err
		}
		if result.ResultSizeEstimate == 0 {
			result.ResultSizeEstimate = page.ResultSizeEstimate
		}
		ids := make([]string, 0, len(page.Messages))
		for _, ref := range page.Messages {
			if ref.ID != "" {
				ids = append(ids, ref.ID)
			}
		}
		if len(ids) > 0 {
			messages, err := s.gmail.BatchGetMessageMetadata(ctx, ids, MetadataHeaders...)
			if err != nil {
				return OnlineSearchResult{}, err
			}
			result.Messages = append(result.Messages, messages...)
		}
		if page.NextPageToken == "" || len(page.Messages) == 0 {
			break
		}
		pageToken = page.NextPageToken
	}
	if len(result.Messages) > limit {
		result.Messages = result.Messages[:limit]
	}
	return result, nil
}

func (b Backfiller) PersistSearchResults(ctx context.Context, account cache.Account, messages []gmail.Message) (SearchPersistenceResult, error) {
	result := SearchPersistenceResult{}
	for _, message := range messages {
		if message.ID == "" {
			continue
		}
		message.ThreadID = firstNonEmpty(message.ThreadID, message.ID)
		labelIDs := mergeLabels(nil, message.LabelIDs)
		decision, err := b.evaluator.Evaluate(ctx, MessageContext{
			AccountID:    account.ID,
			AccountEmail: account.Email,
			From:         headerValue(message.Headers, "From"),
			LabelIDs:     labelIDs,
		})
		if err != nil {
			return SearchPersistenceResult{}, err
		}
		if decision.Excluded {
			result.ExcludedCount++
			continue
		}
		if decision.Persist {
			if err := b.persistMetadata(ctx, account.ID, message, labelIDs); err != nil {
				return SearchPersistenceResult{}, err
			}
			result.MetadataCached++
		}
		if decision.Depth == config.CacheDepthBody || decision.Depth == config.CacheDepthFull {
			result.BodyQueue = append(result.BodyQueue, BodyFetchRequest{
				MessageID: message.ID,
				ThreadID:  message.ThreadID,
				Depth:     decision.Depth,
			})
		}
	}
	return result, nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
