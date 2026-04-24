package ui

import (
	"strings"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
)

func TestKeyOverridesDriveNormalModeActions(t *testing.T) {
	cfg := config.Default()
	cfg.Keys = map[string]string{
		"down":    "n",
		"up":      "p",
		"help":    "h",
		"archive": "A",
	}
	model := New(cfg)

	updated, _ := model.Update(keyMsg("n"))
	model = updated.(Model)
	if model.selectedMessage != 1 {
		t.Fatalf("selected message = %d, want down override", model.selectedMessage)
	}
	updated, _ = model.Update(keyMsg("p"))
	model = updated.(Model)
	if model.selectedMessage != 0 {
		t.Fatalf("selected message = %d, want up override", model.selectedMessage)
	}
	updated, _ = model.Update(keyMsg("h"))
	model = updated.(Model)
	if model.mode != ModeHelp {
		t.Fatalf("mode = %v, want help from override", model.mode)
	}
}

func TestHelpOverlayUsesActiveKeymap(t *testing.T) {
	cfg := config.Default()
	cfg.Keys = map[string]string{
		"down":   "n",
		"up":     "p",
		"search": "S",
	}
	model := New(cfg)
	updated, _ := model.Update(keyMsg("?"))
	model = updated.(Model)

	help := model.View()
	for _, want := range []string{"n  nav", "p  up", "S  search"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
	if strings.Contains(help, "/  search") {
		t.Fatalf("help still shows default search key:\n%s", help)
	}
}
