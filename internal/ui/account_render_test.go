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

func TestAccountRenderingCachedMessagesSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := cachedMessageModel()
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 132, Height: 10})
	model = updated.(Model)

	assertSnapshot(t, model.View(), cachedMessagesSnapshot)
}

func TestCachedMailboxSwitchesMessagesWithActiveLabel(t *testing.T) {
	model := cachedMessageModel()
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 132, Height: 10})
	model = updated.(Model)

	updated, _ = model.Update(keyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(keyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(keyMsg("j"))
	model = updated.(Model)

	if model.selectedLabel != 1 {
		t.Fatalf("selected label = %d, want Sent", model.selectedLabel)
	}
	if len(model.mailbox.Messages) != 1 || model.mailbox.Messages[0].Subject != "Sent cached" {
		t.Fatalf("messages = %#v, want Sent cached message", model.mailbox.Messages)
	}
}

func cachedMessageModel() Model {
	return New(config.Default(), WithMailbox(RealAccountMailbox("me@example.com", []Label{
		{ID: "INBOX", Name: "Inbox", Unread: 2, System: true, CacheDepth: "full"},
		{ID: "SENT", Name: "Sent", System: true, CacheDepth: "body"},
	}, map[string][]Message{
		"INBOX": {
			{
				ID:              "message-1",
				ThreadID:        "thread-1",
				From:            "Ada",
				Address:         "ada@example.com",
				Subject:         "Cached thread",
				Date:            "Apr 24",
				Snippet:         "Cached snippet is loaded from SQLite.",
				Body:            []string{"Cached body"},
				Unread:          true,
				ThreadCount:     3,
				CacheState:      "cached",
				AttachmentCount: 1,
			},
		},
		"SENT": {
			{
				ID:         "message-2",
				ThreadID:   "thread-2",
				From:       "Me",
				Address:    "me@example.com",
				Subject:    "Sent cached",
				Date:       "Apr 23",
				Snippet:    "Sent metadata is loaded from SQLite.",
				CacheState: "metadata",
			},
		},
	})))
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

const cachedMessagesSnapshot = `G&T | me@example.com | Gmail cache
------------------------------------------------------------------------------------------------------------------------------------
Labels                 | Inbox                                      | Reader
> F Inbox          2   | > Ada          Apr 24 Cached thread [3]... | From: Ada <ada@example.com>
  B Sent               |   cached | Cached snippet is loaded fro... | Subject: Cached thread
                       |                                            | Date: Apr 24
                       |                                            |
                       |                                            | Cached body
------------------------------------------------------------------------------------------------------------------------------------
j/k: nav   enter: open   tab: pane   ?: help   q: quit`
