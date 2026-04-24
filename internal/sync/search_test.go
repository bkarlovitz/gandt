package sync

import (
	"context"
	"errors"
	"testing"

	"github.com/bkarlovitz/gandt/internal/gmail"
)

func TestOnlineSearcherPassesQueryAndPaginatesMetadata(t *testing.T) {
	reader := &fakeSearchReader{
		pages: []gmail.ListMessagesPage{
			{
				Messages:      []gmail.MessageRef{{ID: "msg-1"}, {ID: "msg-2"}},
				NextPageToken: "next",
			},
			{Messages: []gmail.MessageRef{{ID: "msg-3"}}},
		},
		metadata: map[string]gmail.Message{
			"msg-1": {ID: "msg-1", Headers: []gmail.MessageHeader{{Name: "Subject", Value: "First"}}},
			"msg-2": {ID: "msg-2", Headers: []gmail.MessageHeader{{Name: "Subject", Value: "Second"}}},
			"msg-3": {ID: "msg-3", Headers: []gmail.MessageHeader{{Name: "Subject", Value: "Third"}}},
		},
	}

	result, err := NewOnlineSearcher(reader).Search(context.Background(), OnlineSearchRequest{
		Query:      "from:ada subject:plan",
		MaxResults: 3,
		PageSize:   2,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Messages) != 3 || result.Messages[2].ID != "msg-3" {
		t.Fatalf("messages = %#v, want three paginated metadata messages", result.Messages)
	}
	if len(reader.listCalls) != 2 || reader.listCalls[0].Query != "from:ada subject:plan" || reader.listCalls[1].PageToken != "next" {
		t.Fatalf("list calls = %#v, want verbatim query and next page token", reader.listCalls)
	}
	if got := reader.metadataHeaders[0]; !equalSearchStrings(got, MetadataHeaders) {
		t.Fatalf("metadata headers = %#v, want %v", got, MetadataHeaders)
	}
}

func TestOnlineSearcherStopsAtLimit(t *testing.T) {
	reader := &fakeSearchReader{
		pages: []gmail.ListMessagesPage{{
			Messages: []gmail.MessageRef{{ID: "msg-1"}, {ID: "msg-2"}, {ID: "msg-3"}},
		}},
		metadata: map[string]gmail.Message{
			"msg-1": {ID: "msg-1"},
			"msg-2": {ID: "msg-2"},
			"msg-3": {ID: "msg-3"},
		},
	}

	result, err := NewOnlineSearcher(reader).Search(context.Background(), OnlineSearchRequest{
		Query:      "label:inbox",
		MaxResults: 2,
		PageSize:   50,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Messages) != 2 || reader.listCalls[0].MaxResults != 2 {
		t.Fatalf("result len=%d max=%d, want limit 2", len(result.Messages), reader.listCalls[0].MaxResults)
	}
}

func TestOnlineSearcherRespectsCancellation(t *testing.T) {
	reader := &fakeSearchReader{listErr: context.Canceled}

	_, err := NewOnlineSearcher(reader).Search(context.Background(), OnlineSearchRequest{Query: "from:ada"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

type fakeSearchReader struct {
	pages           []gmail.ListMessagesPage
	metadata        map[string]gmail.Message
	listCalls       []gmail.ListMessagesOptions
	metadataHeaders [][]string
	listErr         error
}

func (f *fakeSearchReader) ListMessages(ctx context.Context, opts gmail.ListMessagesOptions) (gmail.ListMessagesPage, error) {
	f.listCalls = append(f.listCalls, opts)
	if f.listErr != nil {
		return gmail.ListMessagesPage{}, f.listErr
	}
	index := len(f.listCalls) - 1
	if index >= len(f.pages) {
		return gmail.ListMessagesPage{}, nil
	}
	return f.pages[index], nil
}

func (f *fakeSearchReader) BatchGetMessageMetadata(ctx context.Context, ids []string, headers ...string) ([]gmail.Message, error) {
	f.metadataHeaders = append(f.metadataHeaders, append([]string{}, headers...))
	messages := make([]gmail.Message, 0, len(ids))
	for _, id := range ids {
		messages = append(messages, f.metadata[id])
	}
	return messages, nil
}

func (f *fakeSearchReader) ListHistory(context.Context, gmail.ListHistoryOptions) (gmail.HistoryPage, error) {
	return gmail.HistoryPage{}, nil
}

func (f *fakeSearchReader) GetMessageMetadata(context.Context, string, ...string) (gmail.Message, error) {
	return gmail.Message{}, nil
}

func (f *fakeSearchReader) GetMessageFull(context.Context, string) (gmail.Message, error) {
	return gmail.Message{}, nil
}

func (f *fakeSearchReader) GetThread(context.Context, string, gmail.MessageFormat, ...string) (gmail.Thread, error) {
	return gmail.Thread{}, nil
}

func equalSearchStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
