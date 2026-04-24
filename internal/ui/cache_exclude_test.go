package ui

import (
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
)

func TestCacheExcludeCommandPreviewAndConfirm(t *testing.T) {
	store := &recordingCacheExclusionStore{
		preview: CacheExclusionPreview{MessageCount: 2, BodyCount: 1, AttachmentCount: 1, EstimatedBytes: 2048},
		result:  CacheExclusionResult{DeletedMessages: 2},
	}
	model := New(config.Default(), WithMailbox(RealAccountMailbox("me@example.com", nil)), WithCacheExclusionStore(store))

	updated, cmd := submitTestCommand(model, "cache-exclude sender ada@example.com")
	model = updated.(Model)
	if cmd == nil || !model.loadingCacheExclusion {
		t.Fatalf("preview cmd/loading = %T/%v, want preview command and loading", cmd, model.loadingCacheExclusion)
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if model.pendingCacheExclusion == nil || model.statusMessage != "cache exclusion preview: 2 messages, 1 bodies, 1 attachments, 2.0 KB; y confirm / n cancel" {
		t.Fatalf("pending/status = %#v/%q, want preview prompt", model.pendingCacheExclusion, model.statusMessage)
	}

	updated, confirmCmd := model.Update(keyMsg("y"))
	model = updated.(Model)
	if confirmCmd == nil || !model.loadingCacheExclusion {
		t.Fatalf("confirm cmd/loading = %T/%v, want confirm command and loading", confirmCmd, model.loadingCacheExclusion)
	}
	updated, _ = model.Update(confirmCmd())
	model = updated.(Model)

	if len(store.previewed) != 1 || store.previewed[0].MatchType != "sender" || store.previewed[0].MatchValue != "ada@example.com" {
		t.Fatalf("previewed requests = %#v, want sender ada@example.com", store.previewed)
	}
	if len(store.confirmed) != 1 || store.confirmed[0].Account != "me@example.com" {
		t.Fatalf("confirmed requests = %#v, want current account confirmation", store.confirmed)
	}
	if model.pendingCacheExclusion != nil || model.statusMessage != "cache exclusion saved; purged 2 messages" {
		t.Fatalf("pending/status = %#v/%q, want success", model.pendingCacheExclusion, model.statusMessage)
	}
}

func TestCacheExcludeCommandRejectsInvalidType(t *testing.T) {
	model := New(config.Default(), WithCacheExclusionStore(&recordingCacheExclusionStore{}))

	updated, cmd := submitTestCommand(model, "cache-exclude subject secret")
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("expected no command for invalid type, got %T", cmd)
	}
	if model.statusMessage != "invalid cache exclusion type: subject" {
		t.Fatalf("status = %q, want invalid type", model.statusMessage)
	}
}

func TestCacheExcludeCommandCanCancelPreview(t *testing.T) {
	store := &recordingCacheExclusionStore{
		preview: CacheExclusionPreview{MessageCount: 1, EstimatedBytes: 100},
	}
	model := New(config.Default(), WithCacheExclusionStore(store))

	updated, cmd := submitTestCommand(model, "cache-exclude domain private.example")
	model = updated.(Model)
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	updated, _ = model.Update(keyMsg("n"))
	model = updated.(Model)

	if model.pendingCacheExclusion != nil || len(store.confirmed) != 0 {
		t.Fatalf("pending/confirmed = %#v/%#v, want canceled preview", model.pendingCacheExclusion, store.confirmed)
	}
	if model.statusMessage != "cache exclusion canceled" {
		t.Fatalf("status = %q, want cancel state", model.statusMessage)
	}
}

type recordingCacheExclusionStore struct {
	preview   CacheExclusionPreview
	result    CacheExclusionResult
	previewed []CacheExclusionRequest
	confirmed []CacheExclusionRequest
}

func (s *recordingCacheExclusionStore) PreviewCacheExclusion(request CacheExclusionRequest) (CacheExclusionPreview, error) {
	s.previewed = append(s.previewed, request)
	s.preview.Request = request
	return s.preview, nil
}

func (s *recordingCacheExclusionStore) ConfirmCacheExclusion(request CacheExclusionRequest) (CacheExclusionResult, error) {
	s.confirmed = append(s.confirmed, request)
	s.result.Preview.Request = request
	return s.result, nil
}
