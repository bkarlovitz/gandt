package ui

import (
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
)

func TestCachePurgeCommandDryRunPreview(t *testing.T) {
	var got CachePurgeRequest
	model := New(config.Default(),
		WithMailbox(RealAccountMailbox("me@example.com", nil)),
		WithCachePurgeStore(CachePurgeStoreFunc{
			PlanFn: func(request CachePurgeRequest) (CachePurgePreview, error) {
				got = request
				return CachePurgePreview{Request: request, MessageCount: 3, BodyCount: 2, AttachmentCount: 1, EstimatedBytes: 4096}, nil
			},
		}),
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

func TestCachePurgeCommandPreviewConfirmAndCompact(t *testing.T) {
	var executed CachePurgeRequest
	compacted := false
	model := New(config.Default(),
		WithMailbox(RealAccountMailbox("me@example.com", nil)),
		WithCachePurgeStore(CachePurgeStoreFunc{
			PlanFn: func(request CachePurgeRequest) (CachePurgePreview, error) {
				return CachePurgePreview{Request: request, MessageCount: 2, BodyCount: 1, EstimatedBytes: 2048}, nil
			},
			ExecuteFn: func(request CachePurgeRequest) (CachePurgeResult, error) {
				executed = request
				return CachePurgeResult{DeletedMessages: 2, DeletedAttachmentFiles: 1}, nil
			},
			CompactFn: func() error {
				compacted = true
				return nil
			},
		}),
	)

	updated, cmd := submitTestCommand(model, "cache-purge --label INBOX --older-than 30")
	model = updated.(Model)
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if model.pendingCachePurge == nil || model.statusMessage != "cache purge preview: 2 messages, 1 bodies, 0 attachments, 2.0 KB; y confirm / n cancel" {
		t.Fatalf("pending/status = %#v/%q, want confirmable preview", model.pendingCachePurge, model.statusMessage)
	}
	updated, executeCmd := model.Update(keyMsg("y"))
	model = updated.(Model)
	if executeCmd == nil {
		t.Fatal("expected execute command")
	}
	updated, _ = model.Update(executeCmd())
	model = updated.(Model)
	if executed.LabelID != "INBOX" || executed.OlderThanDays != 30 || model.statusMessage != "cache purge complete: deleted 2 messages, 1 attachment files" {
		t.Fatalf("executed/status = %#v/%q, want purge completion", executed, model.statusMessage)
	}

	updated, compactCmd := submitTestCommand(model, "cache-compact")
	model = updated.(Model)
	updated, _ = model.Update(compactCmd())
	model = updated.(Model)
	if !compacted || model.statusMessage != "cache compact complete" {
		t.Fatalf("compacted/status = %v/%q, want compact completion", compacted, model.statusMessage)
	}
}

func TestCachePurgeCommandCanCancelPreview(t *testing.T) {
	model := New(config.Default(), WithCachePurgeStore(CachePurgeStoreFunc{
		PlanFn: func(request CachePurgeRequest) (CachePurgePreview, error) {
			return CachePurgePreview{Request: request, MessageCount: 1}, nil
		},
	}))

	updated, cmd := submitTestCommand(model, "cache-purge --label INBOX")
	model = updated.(Model)
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	updated, _ = model.Update(keyMsg("n"))
	model = updated.(Model)

	if model.pendingCachePurge != nil || model.statusMessage != "cache purge canceled" {
		t.Fatalf("pending/status = %#v/%q, want canceled purge", model.pendingCachePurge, model.statusMessage)
	}
}

func TestCachePurgeCommandRejectsInvalidFlags(t *testing.T) {
	model := New(config.Default(), WithCachePurgeStore(CachePurgeStoreFunc{
		PlanFn: func(request CachePurgeRequest) (CachePurgePreview, error) {
			t.Fatal("store should not be called")
			return CachePurgePreview{}, nil
		},
	}))

	updated, cmd := submitTestCommand(model, "cache-purge --older-than soon --dry-run")
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected toast dismissal command for invalid flags")
	}
	if model.statusMessage != "--older-than must be positive days" {
		t.Fatalf("status = %q, want invalid older-than error", model.statusMessage)
	}
}
