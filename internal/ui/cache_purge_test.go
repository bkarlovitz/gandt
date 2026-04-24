package ui

import (
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
)

func TestCachePurgeCommandDryRunPreview(t *testing.T) {
	var got CachePurgeRequest
	model := New(config.Default(),
		WithMailbox(RealAccountMailbox("me@example.com", nil)),
		WithCachePurgeStore(CachePurgeStoreFunc(func(request CachePurgeRequest) (CachePurgePreview, error) {
			got = request
			return CachePurgePreview{Request: request, MessageCount: 3, BodyCount: 2, AttachmentCount: 1, EstimatedBytes: 4096}, nil
		})),
	)

	updated, cmd := submitTestCommand(model, "cache-purge --label INBOX --older-than 30d --from ada@example.com --dry-run")
	model = updated.(Model)
	if cmd == nil || !model.loadingCachePurge {
		t.Fatalf("purge cmd/loading = %T/%v, want planning command", cmd, model.loadingCachePurge)
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)

	if got.Account != "me@example.com" || got.LabelID != "INBOX" || got.OlderThanDays != 30 || got.From != "ada@example.com" || !got.DryRun {
		t.Fatalf("request = %#v, want parsed purge flags", got)
	}
	if model.statusMessage != "cache purge dry run: 3 messages, 2 bodies, 1 attachments, 4.0 KB" {
		t.Fatalf("status = %q, want dry-run preview", model.statusMessage)
	}
}

func TestCachePurgeCommandRejectsInvalidFlags(t *testing.T) {
	model := New(config.Default(), WithCachePurgeStore(CachePurgeStoreFunc(func(request CachePurgeRequest) (CachePurgePreview, error) {
		t.Fatal("store should not be called")
		return CachePurgePreview{}, nil
	})))

	updated, cmd := submitTestCommand(model, "cache-purge --older-than soon --dry-run")
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("expected no command for invalid flags, got %T", cmd)
	}
	if model.statusMessage != "--older-than must be positive days" {
		t.Fatalf("status = %q, want invalid older-than error", model.statusMessage)
	}
}
