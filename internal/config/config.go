package config

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

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
	RenderModeRawHTML   RenderMode = "raw_html"
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
	RecentSearchLimit  int           `toml:"recent_search_limit"`
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
			RecentSearchLimit:  20,
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
	if !validTheme(cfg.UI.Theme) {
		return fmt.Errorf("invalid ui.theme %q", cfg.UI.Theme)
	}
	if !validComposeEditor(cfg.UI.ComposeEditor) {
		return fmt.Errorf("invalid ui.compose_editor %q", cfg.UI.ComposeEditor)
	}
	if !validRenderMode(cfg.UI.RenderModeDefault) {
		return fmt.Errorf("invalid ui.render_mode_default %q", cfg.UI.RenderModeDefault)
	}
	if cfg.UI.RecentSearchLimit < 0 {
		return fmt.Errorf("invalid ui.recent_search_limit %d", cfg.UI.RecentSearchLimit)
	}
	if !validCacheDepth(cfg.Cache.Defaults.Depth) {
		return fmt.Errorf("invalid cache.defaults.depth %q", cfg.Cache.Defaults.Depth)
	}
	if !validAttachmentRule(cfg.Cache.Defaults.AttachmentRule) {
		return fmt.Errorf("invalid cache.defaults.attachment_rule %q", cfg.Cache.Defaults.AttachmentRule)
	}
	if err := ValidateKeyOverrides(cfg.Keys); err != nil {
		return err
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

func ValidateKeyOverrides(overrides map[string]string) error {
	if len(overrides) == 0 {
		return nil
	}
	bindings := defaultKeyBindings()
	for action, value := range overrides {
		if _, ok := bindings[action]; !ok {
			return fmt.Errorf("unknown [keys].%s (valid actions: %s)", action, strings.Join(sortedKeys(bindings), ", "))
		}
		keys := splitKeyList(value)
		if len(keys) == 0 {
			return fmt.Errorf("invalid [keys].%s: at least one key is required", action)
		}
		for _, keyName := range keys {
			if !validKeyName(keyName) {
				return fmt.Errorf("invalid [keys].%s key %q", action, keyName)
			}
		}
		bindings[action] = keys
	}

	seen := map[string]string{}
	for action, keys := range bindings {
		for _, keyName := range keys {
			if other, ok := seen[keyName]; ok {
				return fmt.Errorf("key %q is assigned to both [keys].%s and [keys].%s", keyName, other, action)
			}
			seen[keyName] = action
		}
	}
	return nil
}

func defaultKeyBindings() map[string][]string {
	return map[string][]string{
		"up":                      {"k", "up"},
		"down":                    {"j", "down"},
		"top":                     {"g"},
		"bottom":                  {"G"},
		"open":                    {"enter"},
		"next_pane":               {"tab"},
		"quit":                    {"q", "esc", "ctrl+c"},
		"help":                    {"?"},
		"search":                  {"/"},
		"compose":                 {"c"},
		"command":                 {":"},
		"thread_next_message":     {"J"},
		"thread_previous_message": {"K"},
		"next_thread":             {"N"},
		"previous_thread":         {"P"},
		"render_mode":             {"V"},
		"browser":                 {"B"},
		"quotes":                  {"z"},
		"refresh":                 {"ctrl+r"},
		"reply":                   {"r"},
		"reply_all":               {"R"},
		"forward":                 {"f"},
		"archive":                 {"e"},
		"trash":                   {"#"},
		"spam":                    {"!"},
		"star":                    {"s"},
		"unread":                  {"u"},
		"undo":                    {"U"},
		"mute":                    {"m"},
		"label_add":               {"+"},
		"label_remove":            {"-"},
		"account_switcher":        {"ctrl+a"},
	}
}

func splitKeyList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func validKeyName(value string) bool {
	if len([]rune(value)) == 1 {
		return true
	}
	switch value {
	case "up", "down", "left", "right", "enter", "tab", "esc", "backspace", "delete",
		"ctrl+a", "ctrl+b", "ctrl+c", "ctrl+d", "ctrl+e", "ctrl+f", "ctrl+h", "ctrl+j", "ctrl+k", "ctrl+l", "ctrl+n", "ctrl+p", "ctrl+r", "ctrl+s", "ctrl+t", "ctrl+u", "ctrl+w", "ctrl+x", "ctrl+y", "ctrl+z", "ctrl+/", "ctrl+_", "ctrl+?":
		return true
	default:
		return false
	}
}

func sortedKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func validTheme(value Theme) bool {
	switch value {
	case ThemeDark, ThemeLight, ThemeAuto:
		return true
	default:
		return false
	}
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
	case RenderModePlaintext, RenderModeHTMLText, RenderModeRawHTML, RenderModeGlamour:
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
