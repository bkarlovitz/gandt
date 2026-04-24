package compose

import (
	"testing"
	"time"
)

func TestAutosaverDueEveryThirtySecondsWhenDirty(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	autosaver := NewAutosaver(0)
	if autosaver.Interval != DefaultAutosaveInterval {
		t.Fatalf("interval = %s, want default", autosaver.Interval)
	}
	if autosaver.Due(now) {
		t.Fatal("clean autosaver should not be due")
	}

	autosaver = autosaver.MarkDirty()
	if !autosaver.Due(now) {
		t.Fatal("dirty autosaver with no prior save should be due")
	}

	autosaver = autosaver.MarkSaved(now).MarkDirty()
	if autosaver.Due(now.Add(29 * time.Second)) {
		t.Fatal("autosave should wait for interval")
	}
	if !autosaver.Due(now.Add(30 * time.Second)) {
		t.Fatal("autosave should be due at interval")
	}
}
