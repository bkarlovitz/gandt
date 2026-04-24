package ui

import (
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestMailboxGoldenWideLayout(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := sizedModel(132, 16)

	assertSnapshot(t, model.View(), wideSnapshot)
}

func TestMailboxGoldenMediumLayout(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := sizedModel(100, 14)

	assertSnapshot(t, model.View(), mediumSnapshot)
}

func TestMailboxGoldenNarrowLayout(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := sizedModel(72, 14)

	assertSnapshot(t, model.View(), narrowSnapshot)
}

func sizedModel(width, height int) Model {
	model := New(config.Default())
	updated, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return updated.(Model)
}

func assertSnapshot(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("snapshot mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

const wideSnapshot = `G&T | work: me@work.com | fake inbox | no network
------------------------------------------------------------------------------------------------------------------------------------
Labels                 | Inbox                                      | Reader
> Inbox           42   | > Alice         9:21 Re: Q4 plan [2] *     | From: Alice <alice@example.com>
  Starred          3   |   On Q4, I think we should focus on mig... | Subject: Re: Q4 plan
  Sent                 |   Bob           9:10 Invoice #4132         | Date: Thu Apr 23 9:21
  Drafts           1   |   Attached the updated invoice for the ... |
  Important        7   |   Carol         8:55 Lunch next week?      | Hey - on Q4, I think we should focus on the following:
  Spam                 |   I can do Tuesday or Thursday near the... |
  Trash                |   Delta Alerts  8:31 Build pipeline rec... | 1. Migration prep
-- Labels --           |   The nightly Linux build is green agai... | 2. Hiring pipeline
  receipts         4   |                                            | 3. Customer readiness notes
  travel               |                                            |
  work            12   |                                            | ...
------------------------------------------------------------------------------------------------------------------------------------
?: help   /: search   c: compose   : command   q/esc: quit`

const mediumSnapshot = `G&T | work: me@work.com | fake inbox | no network
----------------------------------------------------------------------------------------------------
Inbox                                  | Reader
> Alice         9:21 Re: Q4 plan [2] * | From: Alice <alice@example.com>
  On Q4, I think we should focus on... | Subject: Re: Q4 plan
  Bob           9:10 Invoice #4132     | Date: Thu Apr 23 9:21
  Attached the updated invoice for ... |
  Carol         8:55 Lunch next wee... | Hey - on Q4, I think we should focus on the following:
  I can do Tuesday or Thursday near... |
  Delta Alerts  8:31 Build pipeline... | 1. Migration prep
  The nightly Linux build is green ... | 2. Hiring pipeline
                                       | ...
----------------------------------------------------------------------------------------------------
?: help   /: search   c: compose   : command   q/esc: quit`

const narrowSnapshot = `G&T | work: me@work.com | fake inbox | no network
------------------------------------------------------------------------
Inbox
> Alice         9:21 Re: Q4 plan [2] *
  On Q4, I think we should focus on migration prep and hiring.
  Bob           9:10 Invoice #4132
  Attached the updated invoice for the April services window.
  Carol         8:55 Lunch next week?
  I can do Tuesday or Thursday near the office.
  Delta Alerts  8:31 Build pipeline recovered [4]
  The nightly Linux build is green again after retry.
------------------------------------------------------------------------
?: help   /: search   c: compose   : command   q/esc: quit`
