package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingConfigReturnsDefaults(t *testing.T) {
	cfg, err := Load(Paths{ConfigFile: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	assertEqual(t, string(cfg.UI.Theme), "dark")
	assertEqual(t, string(cfg.UI.ComposeEditor), "external")
	assertEqual(t, string(cfg.UI.RenderModeDefault), "plaintext")
	assertEqual(t, cfg.Sync.PollActiveSeconds, 60)
	assertEqual(t, string(cfg.Cache.Defaults.Depth), "full")
	assertEqual(t, string(cfg.Cache.Defaults.AttachmentRule), "under_size")
	assertEqual(t, cfg.Paths.Downloads, "~/Downloads")
}

func TestLoadValidConfig(t *testing.T) {
	path := writeConfig(t, `
[ui]
theme = "light"
compose_editor = "inline"
render_mode_default = "glamour"
render_url_footnotes = false

[sync]
poll_active_seconds = 30
poll_idle_seconds = 120
backfill_limit_per_label = 250

[cache.defaults]
depth = "body"
retention_days = 45
attachment_rule = "none"
attachment_max_mb = 5
total_budget_mb = 1000

[[cache.policies]]
account = "work"
label = "receipts"
depth = "full"
retention_days = 1825
attachment_rule = "all"
attachment_max_mb = 20

[[cache.exclusions]]
account = "personal"
match_type = "label"
match_value = "private"

[accounts.work]
email = "me@example.com"
color = "#4287f5"

[keys]
archive = "e"
trash = "#"

[paths]
downloads = "~/mail-downloads"
`)

	cfg, err := Load(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	assertEqual(t, string(cfg.UI.Theme), "light")
	assertEqual(t, string(cfg.UI.ComposeEditor), "inline")
	assertEqual(t, string(cfg.UI.RenderModeDefault), "glamour")
	assertEqual(t, cfg.UI.RenderURLFootnotes, false)
	assertEqual(t, cfg.Sync.PollActiveSeconds, 30)
	assertEqual(t, string(cfg.Cache.Defaults.Depth), "body")
	assertEqual(t, len(cfg.Cache.Policies), 1)
	assertEqual(t, string(cfg.Cache.Policies[0].AttachmentRule), "all")
	assertEqual(t, len(cfg.Cache.Exclusions), 1)
	assertEqual(t, cfg.Accounts["work"].Email, "me@example.com")
	assertEqual(t, cfg.Accounts["work"].Color, "#4287f5")
	assertEqual(t, cfg.Keys["trash"], "#")
	assertEqual(t, cfg.Paths.Downloads, "~/mail-downloads")
}

func TestLoadRejectsInvalidEnums(t *testing.T) {
	tests := map[string]string{
		"compose editor": `
[ui]
compose_editor = "terminal"
`,
		"render mode": `
[ui]
render_mode_default = "rich"
`,
		"cache depth": `
[cache.defaults]
depth = "everything"
`,
		"attachment rule": `
[cache.defaults]
attachment_rule = "small"
`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(Paths{ConfigFile: writeConfig(t, body)})
			if err == nil {
				t.Fatal("expected invalid enum error")
			}
		})
	}
}

func TestLoadConfigOverridesKeepUnspecifiedDefaults(t *testing.T) {
	path := writeConfig(t, `
[ui]
compose_editor = "inline"

[cache.defaults]
depth = "metadata"
`)

	cfg, err := Load(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	assertEqual(t, string(cfg.UI.ComposeEditor), "inline")
	assertEqual(t, string(cfg.UI.RenderModeDefault), "plaintext")
	assertEqual(t, string(cfg.Cache.Defaults.Depth), "metadata")
	assertEqual(t, cfg.Cache.Defaults.RetentionDays, 90)
	assertEqual(t, string(cfg.Cache.Defaults.AttachmentRule), "under_size")
	assertEqual(t, cfg.Cache.Defaults.TotalBudgetMB, 2000)
}

func TestLoadReloadsCachePolicyChanges(t *testing.T) {
	path := writeConfig(t, `
[cache.defaults]
depth = "metadata"
retention_days = 30
attachment_rule = "none"

[[cache.policies]]
account = "me@example.com"
label = "Receipts"
depth = "body"
retention_days = 90
attachment_rule = "none"

[[cache.exclusions]]
match_type = "domain"
match_value = "private.example"
`)

	first, err := Load(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("load first config: %v", err)
	}
	assertEqual(t, string(first.Cache.Defaults.Depth), "metadata")
	assertEqual(t, string(first.Cache.Policies[0].Depth), "body")
	assertEqual(t, first.Cache.Policies[0].RetentionDays, 90)
	assertEqual(t, first.Cache.Exclusions[0].MatchValue, "private.example")

	if err := os.WriteFile(path, []byte(strings.TrimSpace(`
[cache.defaults]
depth = "full"
retention_days = 365
attachment_rule = "under_size"
attachment_max_mb = 5

[[cache.policies]]
account = "me@example.com"
label = "Receipts"
depth = "full"
retention_days = 1825
attachment_rule = "all"
`)+"\n"), 0o600); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}

	second, err := Load(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("load second config: %v", err)
	}
	assertEqual(t, string(second.Cache.Defaults.Depth), "full")
	assertEqual(t, second.Cache.Defaults.AttachmentMaxMB, 5)
	assertEqual(t, string(second.Cache.Policies[0].Depth), "full")
	assertEqual(t, second.Cache.Policies[0].RetentionDays, 1825)
	assertEqual(t, len(second.Cache.Exclusions), 0)
}

func TestInitFileLoggerCreatesLogFile(t *testing.T) {
	root := t.TempDir()
	paths := Paths{
		ConfigDir: filepath.Join(root, "config"),
		DataDir:   filepath.Join(root, "data"),
		LogDir:    filepath.Join(root, "data", "logs"),
		LogFile:   filepath.Join(root, "data", "logs", "gandt.log"),
	}

	logFile, err := InitFileLogger(paths, "test-version")
	if err != nil {
		t.Fatalf("init logger: %v", err)
	}
	if err := logFile.Close(); err != nil {
		t.Fatalf("close logger: %v", err)
	}

	info, err := os.Stat(paths.LogFile)
	if err != nil {
		t.Fatalf("stat log file: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("log file permissions are too broad: %s", info.Mode().Perm())
	}

	body, err := os.ReadFile(paths.LogFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	logs := string(body)
	if !strings.Contains(logs, `msg=startup`) || !strings.Contains(logs, `version=test-version`) {
		t.Fatalf("startup metadata missing from log: %s", logs)
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}
