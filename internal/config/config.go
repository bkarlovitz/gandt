package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Theme string
type ComposeEditor string
type RenderMode string
type CacheDepth string
type AttachmentRule string

const (
	ThemeDark  Theme = "dark"
	ThemeLight Theme = "light"
	ThemeAuto  Theme = "auto"

	ComposeEditorExternal ComposeEditor = "external"
	ComposeEditorInline   ComposeEditor = "inline"

	RenderModePlaintext RenderMode = "plaintext"
	RenderModeHTMLText  RenderMode = "html2text"
	RenderModeGlamour   RenderMode = "glamour"

	CacheDepthNone     CacheDepth = "none"
	CacheDepthMetadata CacheDepth = "metadata"
	CacheDepthBody     CacheDepth = "body"
	CacheDepthFull     CacheDepth = "full"

	AttachmentRuleNone      AttachmentRule = "none"
	AttachmentRuleUnderSize AttachmentRule = "under_size"
	AttachmentRuleAll       AttachmentRule = "all"
)

type Config struct {
	UI       UIConfig                 `toml:"ui"`
	Sync     SyncConfig               `toml:"sync"`
	Cache    CacheConfig              `toml:"cache"`
	Accounts map[string]AccountConfig `toml:"accounts"`
	Keys     map[string]string        `toml:"keys"`
	Paths    PathConfig               `toml:"paths"`
}

type UIConfig struct {
	Theme              Theme         `toml:"theme"`
	ComposeEditor      ComposeEditor `toml:"compose_editor"`
	RenderModeDefault  RenderMode    `toml:"render_mode_default"`
	RenderURLFootnotes bool          `toml:"render_url_footnotes"`
}

type SyncConfig struct {
	PollActiveSeconds     int `toml:"poll_active_seconds"`
	PollIdleSeconds       int `toml:"poll_idle_seconds"`
	BackfillLimitPerLabel int `toml:"backfill_limit_per_label"`
}

type CacheConfig struct {
	Defaults   CacheDefaults    `toml:"defaults"`
	Policies   []CachePolicy    `toml:"policies"`
	Exclusions []CacheExclusion `toml:"exclusions"`
}

type CacheDefaults struct {
	Depth           CacheDepth     `toml:"depth"`
	RetentionDays   int            `toml:"retention_days"`
	AttachmentRule  AttachmentRule `toml:"attachment_rule"`
	AttachmentMaxMB int            `toml:"attachment_max_mb"`
	TotalBudgetMB   int            `toml:"total_budget_mb"`
}

type CachePolicy struct {
	Account         string         `toml:"account"`
	Label           string         `toml:"label"`
	Depth           CacheDepth     `toml:"depth"`
	RetentionDays   int            `toml:"retention_days"`
	AttachmentRule  AttachmentRule `toml:"attachment_rule"`
	AttachmentMaxMB int            `toml:"attachment_max_mb"`
}

type CacheExclusion struct {
	Account    string `toml:"account"`
	MatchType  string `toml:"match_type"`
	MatchValue string `toml:"match_value"`
}

type AccountConfig struct {
	Email string `toml:"email"`
	ID    string `toml:"id"`
	Color string `toml:"color"`
}

type PathConfig struct {
	Downloads string `toml:"downloads"`
}

// Default returns the PRD-backed runtime configuration defaults.
func Default() Config {
	return Config{
		UI: UIConfig{
			Theme:              ThemeDark,
			ComposeEditor:      ComposeEditorExternal,
			RenderModeDefault:  RenderModePlaintext,
			RenderURLFootnotes: true,
		},
		Sync: SyncConfig{
			PollActiveSeconds:     60,
			PollIdleSeconds:       300,
			BackfillLimitPerLabel: 5000,
		},
		Cache: CacheConfig{
			Defaults: CacheDefaults{
				Depth:           CacheDepthFull,
				RetentionDays:   90,
				AttachmentRule:  AttachmentRuleUnderSize,
				AttachmentMaxMB: 10,
				TotalBudgetMB:   2000,
			},
		},
		Accounts: map[string]AccountConfig{},
		Keys:     map[string]string{},
		Paths: PathConfig{
			Downloads: "~/Downloads",
		},
	}
}

// Load reads config.toml if present, applying values over PRD defaults.
func Load(paths Paths) (Config, error) {
	cfg := Default()
	if paths.ConfigFile == "" {
		return cfg, errors.New("config file path is empty")
	}

	if _, err := os.Stat(paths.ConfigFile); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if _, err := toml.DecodeFile(paths.ConfigFile, &cfg); err != nil {
		return cfg, err
	}

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func (cfg Config) Validate() error {
	if !validComposeEditor(cfg.UI.ComposeEditor) {
		return fmt.Errorf("invalid ui.compose_editor %q", cfg.UI.ComposeEditor)
	}
	if !validRenderMode(cfg.UI.RenderModeDefault) {
		return fmt.Errorf("invalid ui.render_mode_default %q", cfg.UI.RenderModeDefault)
	}
	if !validCacheDepth(cfg.Cache.Defaults.Depth) {
		return fmt.Errorf("invalid cache.defaults.depth %q", cfg.Cache.Defaults.Depth)
	}
	if !validAttachmentRule(cfg.Cache.Defaults.AttachmentRule) {
		return fmt.Errorf("invalid cache.defaults.attachment_rule %q", cfg.Cache.Defaults.AttachmentRule)
	}

	for i, policy := range cfg.Cache.Policies {
		if !validCacheDepth(policy.Depth) {
			return fmt.Errorf("invalid cache.policies[%d].depth %q", i, policy.Depth)
		}
		if !validAttachmentRule(policy.AttachmentRule) {
			return fmt.Errorf("invalid cache.policies[%d].attachment_rule %q", i, policy.AttachmentRule)
		}
	}

	return nil
}

func validComposeEditor(value ComposeEditor) bool {
	switch value {
	case ComposeEditorExternal, ComposeEditorInline:
		return true
	default:
		return false
	}
}

func validRenderMode(value RenderMode) bool {
	switch value {
	case RenderModePlaintext, RenderModeHTMLText, RenderModeGlamour:
		return true
	default:
		return false
	}
}

func validCacheDepth(value CacheDepth) bool {
	switch value {
	case CacheDepthNone, CacheDepthMetadata, CacheDepthBody, CacheDepthFull:
		return true
	default:
		return false
	}
}

func validAttachmentRule(value AttachmentRule) bool {
	switch value {
	case AttachmentRuleNone, AttachmentRuleUnderSize, AttachmentRuleAll:
		return true
	default:
		return false
	}
}
