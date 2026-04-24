package ui

import (
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
)

func TestCacheWipeRequiresTwoConfirmations(t *testing.T) {
	calls := 0
	model := New(config.Default(), WithCacheWipeStore(CacheWipeStoreFunc(func() (CacheWipeResult, error) {
		calls++
		return CacheWipeResult{DatabaseFilesRemoved: 3, AttachmentFilesRemoved: 2}, nil
	})))

	updated, cmd := submitTestCommand(model, "cache-wipe")
	model = updated.(Model)
	if cmd != nil || model.pendingCacheWipeStep != 1 || model.statusMessage != "cache wipe confirmation 1/2: press y to continue" {
		t.Fatalf("cmd/step/status = %T/%d/%q, want first confirmation", cmd, model.pendingCacheWipeStep, model.statusMessage)
	}
	updated, cmd = model.Update(keyMsg("y"))
	model = updated.(Model)
	if cmd != nil || model.pendingCacheWipeStep != 2 || calls != 0 {
		t.Fatalf("cmd/step/calls = %T/%d/%d, want second confirmation without wipe", cmd, model.pendingCacheWipeStep, calls)
	}
	updated, cmd = model.Update(keyMsg("y"))
	model = updated.(Model)
	if cmd == nil || !model.loadingCacheWipe {
		t.Fatalf("cmd/loading = %T/%v, want wipe command", cmd, model.loadingCacheWipe)
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)

	if calls != 1 || model.pendingCacheWipeStep != 0 || model.statusMessage != "cache wipe complete: removed 3 database files, 2 attachment files" {
		t.Fatalf("calls/step/status = %d/%d/%q, want completed wipe", calls, model.pendingCacheWipeStep, model.statusMessage)
	}
}

func TestCacheWipeCanCancel(t *testing.T) {
	calls := 0
	model := New(config.Default(), WithCacheWipeStore(CacheWipeStoreFunc(func() (CacheWipeResult, error) {
		calls++
		return CacheWipeResult{}, nil
	})))

	updated, _ := submitTestCommand(model, "cache-wipe")
	model = updated.(Model)
	updated, _ = model.Update(keyMsg("n"))
	model = updated.(Model)

	if calls != 0 || model.pendingCacheWipeStep != 0 || model.statusMessage != "cache wipe canceled" {
		t.Fatalf("calls/step/status = %d/%d/%q, want canceled wipe", calls, model.pendingCacheWipeStep, model.statusMessage)
	}
}
