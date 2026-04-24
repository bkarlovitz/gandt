package ui

import (
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestAccountRenderingNoAccountSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := New(config.Default(), WithMailbox(NoAccountMailbox()))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	model = updated.(Model)

	assertSnapshot(t, model.View(), noAccountSnapshot)
}

func TestAccountRenderingBootstrappingSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := New(config.Default(), WithMailbox(BootstrappingMailbox()))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	model = updated.(Model)

	assertSnapshot(t, model.View(), bootstrappingSnapshot)
}

func TestAccountRenderingOneAccountSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := New(config.Default(), WithMailbox(RealAccountMailbox("me@example.com", []Label{
		{Name: "Inbox", Unread: 2, System: true, CacheDepth: "full"},
		{Name: "Sent", System: true, CacheDepth: "body"},
		{Name: "Receipts", Unread: 1, CacheDepth: "metadata"},
	})))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 132, Height: 10})
	model = updated.(Model)

	assertSnapshot(t, model.View(), oneAccountSnapshot)
}

const noAccountSnapshot = `G&T | no accounts configured
--------------------------------------------------------------------------------
No labels                              | Reader
                                       |
No cached messages                     | No message selected
--------------------------------------------------------------------------------
j/k: nav   enter: open   tab: pane   ?: help   q: quit`

const bootstrappingSnapshot = `G&T | work: me@work.com | bootstrapping account
--------------------------------------------------------------------------------
Inbox                                  | Reader
> Alice         9:21 Re: Q4 plan [2] * | From: Alice <alice@example.com>
  On Q4, I think we should focus on... | Subject: Re: Q4 plan
  Bob           9:10 Invoice #4132     | Date: Thu Apr 23 9:21
  Attached the updated invoice for ... |
...                                    | ...
--------------------------------------------------------------------------------
j/k: nav   enter: open   tab: pane   ?: help   q: quit`

const oneAccountSnapshot = `G&T | me@example.com | Gmail cache
------------------------------------------------------------------------------------------------------------------------------------
Labels                 | Inbox                                      | Reader
> F Inbox          2   |                                            |
  B Sent               | No cached messages                         | No message selected
-- Labels --           |                                            |
  M Receipts       1   |                                            |
------------------------------------------------------------------------------------------------------------------------------------
j/k: nav   enter: open   tab: pane   ?: help   q: quit`
