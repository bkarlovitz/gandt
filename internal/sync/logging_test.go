package sync

import (
	"context"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/gmail"
)

func TestSyncLogsStructuredEvents(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccountWithHistory(t, db, "100")
	events := []loggedSyncEvent{}
	logger := LoggerFunc(func(event string, fields map[string]any) {
		copied := map[string]any{}
		for key, value := range fields {
			copied[key] = value
		}
		events = append(events, loggedSyncEvent{Name: event, Fields: copied})
	})
	client := newFakeHistoryReader()
	client.historyPages = []gmail.HistoryPage{{HistoryID: "101"}}

	if _, err := NewDeltaSynchronizer(db, config.Default(), client, WithLogger(logger)).Sync(ctx, account); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(events) < 4 {
		t.Fatalf("events = %#v, want start/delta/success events", events)
	}
	assertLoggedEvent(t, events, "sync_start", account.ID)
	assertLoggedEvent(t, events, "delta_sync_start", account.ID)
	assertLoggedEvent(t, events, "delta_sync_success", account.ID)
	assertLoggedEvent(t, events, "sync_success", account.ID)
}

func TestSyncLogsFailuresWithoutMessageBodies(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccountWithHistory(t, db, "100")
	events := []loggedSyncEvent{}
	logger := LoggerFunc(func(event string, fields map[string]any) {
		events = append(events, loggedSyncEvent{Name: event, Fields: fields})
	})
	client := newFakeHistoryReader()
	client.historyPages = []gmail.HistoryPage{{
		HistoryID: "101",
		Records: []gmail.HistoryRecord{{
			MessagesAdded: []gmail.HistoryMessageChange{{Message: gmail.MessageRef{ID: "missing", ThreadID: "thread-1"}}},
		}},
	}}

	if _, err := NewDeltaSynchronizer(db, config.Default(), client, WithLogger(logger)).Sync(ctx, account); err == nil {
		t.Fatal("sync unexpectedly succeeded")
	}
	failure := findLoggedEvent(events, "sync_failure")
	if failure == nil {
		t.Fatalf("events = %#v, want sync_failure", events)
	}
	if _, ok := failure.Fields["body"]; ok {
		t.Fatalf("failure fields include message body: %#v", failure.Fields)
	}
	if failure.Fields["account_id"] != account.ID || failure.Fields["email"] != account.Email {
		t.Fatalf("failure fields = %#v, want account identifiers", failure.Fields)
	}
}

type loggedSyncEvent struct {
	Name   string
	Fields map[string]any
}

func assertLoggedEvent(t *testing.T, events []loggedSyncEvent, name string, accountID string) {
	t.Helper()
	event := findLoggedEvent(events, name)
	if event == nil {
		t.Fatalf("events = %#v, want %s", events, name)
	}
	if event.Fields["account_id"] != accountID {
		t.Fatalf("%s account_id = %#v, want %s", name, event.Fields["account_id"], accountID)
	}
	if _, ok := event.Fields["duration_ms"]; name != "sync_start" && name != "delta_sync_start" && !ok {
		t.Fatalf("%s fields = %#v, want duration_ms", name, event.Fields)
	}
}

func findLoggedEvent(events []loggedSyncEvent, name string) *loggedSyncEvent {
	for i := range events {
		if events[i].Name == name {
			return &events[i]
		}
	}
	return nil
}
