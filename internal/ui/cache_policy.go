package ui

import (
	"fmt"
	"strings"
)

type CachePolicyTable struct {
	Rows []CachePolicyRow
}

type CachePolicyRow struct {
	AccountID       string
	AccountEmail    string
	LabelID         string
	LabelName       string
	Explicit        bool
	Depth           string
	RetentionDays   *int
	AttachmentRule  string
	AttachmentMaxMB *int
}

type CachePolicyStore interface {
	LoadCachePolicies() (CachePolicyTable, error)
	SaveCachePolicy(CachePolicyRow) (CachePolicyRow, error)
	ResetCachePolicy(CachePolicyRow) (CachePolicyRow, error)
}

type CachePolicyStoreFunc struct {
	LoadFn  func() (CachePolicyTable, error)
	SaveFn  func(CachePolicyRow) (CachePolicyRow, error)
	ResetFn func(CachePolicyRow) (CachePolicyRow, error)
}

func (fn CachePolicyStoreFunc) LoadCachePolicies() (CachePolicyTable, error) {
	return fn.LoadFn()
}

func (fn CachePolicyStoreFunc) SaveCachePolicy(row CachePolicyRow) (CachePolicyRow, error) {
	return fn.SaveFn(row)
}

func (fn CachePolicyStoreFunc) ResetCachePolicy(row CachePolicyRow) (CachePolicyRow, error) {
	return fn.ResetFn(row)
}

func (m Model) renderCachePolicyEditor() string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	height := m.height
	if height <= 0 {
		height = 30
	}

	lines := []string{
		fit("G&T | cache policy", width),
		strings.Repeat("-", width),
	}
	lines = append(lines, cachePolicyTableLines(m.cachePolicyTable.Rows, m.selectedCachePolicy, width, height-6)...)
	if m.statusMessage != "" {
		lines = append(lines, "", fit(m.statusMessage, width))
	}
	lines = append(lines, strings.Repeat("-", width), fit("j/k: row   d: depth   t: retention   a: attachments   s: save   x: reset   Esc: cancel", width))
	return trimRightLines(strings.Join(limitLines(lines, height, width), "\n"))
}

func cachePolicyTableLines(rows []CachePolicyRow, selected int, width int, maxRows int) []string {
	lines := []string{fit("P  Account              Label          Depth      Retention  Attachments", width)}
	if len(rows) == 0 {
		return append(lines, fit("No cache policies", width))
	}
	if maxRows < 2 {
		maxRows = 2
	}
	start := 0
	if selected >= maxRows-1 {
		start = selected - (maxRows - 2)
	}
	end := start + maxRows - 1
	if end > len(rows) {
		end = len(rows)
	}
	for i := start; i < end; i++ {
		row := rows[i]
		prefix := "  "
		if i == selected {
			prefix = "> "
		}
		explicit := " "
		if row.Explicit {
			explicit = "*"
		}
		lines = append(lines, fit(fmt.Sprintf("%s%s %-20s %-14s %-10s %-10s %s",
			prefix,
			explicit,
			row.AccountEmail,
			firstNonEmpty(row.LabelName, row.LabelID),
			row.Depth,
			retentionText(row.RetentionDays),
			attachmentPolicyText(row),
		), width))
	}
	if end < len(rows) {
		lines = append(lines, fit("...", width))
	}
	return lines
}

func retentionText(days *int) string {
	if days == nil {
		return "none"
	}
	return fmt.Sprintf("%dd", *days)
}

func attachmentPolicyText(row CachePolicyRow) string {
	switch row.AttachmentRule {
	case "under_size":
		if row.AttachmentMaxMB == nil {
			return "under_size"
		}
		return fmt.Sprintf("under %dMB", *row.AttachmentMaxMB)
	case "all":
		return "all"
	default:
		return "none"
	}
}

func cyclePolicyDepth(row CachePolicyRow) CachePolicyRow {
	depths := []string{"none", "metadata", "body", "full"}
	row.Depth = nextString(depths, row.Depth)
	if row.Depth == "none" {
		row.AttachmentRule = "none"
		row.AttachmentMaxMB = nil
	}
	return row
}

func cyclePolicyRetention(row CachePolicyRow) CachePolicyRow {
	values := []*int{nil, intValue(30), intValue(90), intValue(365)}
	current := 0
	if row.RetentionDays != nil {
		for i, value := range values {
			if value != nil && *value == *row.RetentionDays {
				current = i
				break
			}
		}
	}
	row.RetentionDays = values[(current+1)%len(values)]
	return row
}

func cyclePolicyAttachment(row CachePolicyRow) CachePolicyRow {
	rules := []string{"none", "under_size", "all"}
	row.AttachmentRule = nextString(rules, row.AttachmentRule)
	switch row.AttachmentRule {
	case "under_size":
		if row.AttachmentMaxMB == nil {
			row.AttachmentMaxMB = intValue(10)
		}
	case "all", "none":
		row.AttachmentMaxMB = nil
	}
	return row
}

func adjustPolicyAttachmentMax(row CachePolicyRow, delta int) CachePolicyRow {
	if row.AttachmentRule != "under_size" {
		return row
	}
	value := 10
	if row.AttachmentMaxMB != nil {
		value = *row.AttachmentMaxMB
	}
	value += delta
	if value < 1 {
		value = 1
	}
	row.AttachmentMaxMB = &value
	return row
}

func nextString(values []string, current string) string {
	for i, value := range values {
		if value == current {
			return values[(i+1)%len(values)]
		}
	}
	return values[0]
}

func intValue(value int) *int {
	return &value
}
