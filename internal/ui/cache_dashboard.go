package ui

import (
	"fmt"
	"strings"
)

func (m Model) renderCacheDashboard() string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	height := m.height
	if height <= 0 {
		height = 30
	}

	dashboard := m.cacheDashboard
	lines := []string{
		fit(fmt.Sprintf("G&T | cache dashboard | total %s | sqlite %s", formatBytes(dashboard.TotalBytes), formatBytes(dashboard.SQLiteBytes)), width),
		strings.Repeat("-", width),
	}

	lines = append(lines,
		fit(fmt.Sprintf("Messages %d | bodies %d | attachments %d cached %d | FTS %d rows %s",
			dashboard.MessageCount,
			dashboard.BodyCount,
			dashboard.AttachmentCount,
			dashboard.CachedAttachmentCount,
			dashboard.FTSRows,
			formatBytes(dashboard.FTSBytes),
		), width),
		"",
	)

	lines = append(lines, cacheTable("Accounts", []string{"Account", "Msgs", "Bodies", "Att", "Bytes"}, accountRows(dashboard.Accounts), width, 6)...)
	lines = append(lines, "")
	lines = append(lines, cacheTable("Labels", []string{"P", "Label", "Msgs", "Bodies", "Att", "Bytes"}, labelRows(dashboard.Labels), width, 8)...)
	lines = append(lines, "")
	lines = append(lines, cacheTable("Age", []string{"Bucket", "Msgs", "Bodies", "Att", "Bytes"}, ageRows(dashboard.Ages), width, 7)...)
	lines = append(lines, "")
	lines = append(lines, cacheTable("Rows", []string{"Table", "Rows"}, rowRows(dashboard.Rows), width, 8)...)

	if m.statusMessage != "" {
		lines = append(lines, "", fit(m.statusMessage, width))
	}
	lines = append(lines, strings.Repeat("-", width), fit("Esc: mailbox   q: quit", width))
	return trimRightLines(strings.Join(limitLines(lines, height, width), "\n"))
}

func accountRows(accounts []CacheDashboardAccount) [][]string {
	rows := make([][]string, 0, len(accounts))
	for _, account := range accounts {
		rows = append(rows, []string{
			account.Email,
			fmt.Sprintf("%d", account.MessageCount),
			fmt.Sprintf("%d", account.BodyCount),
			fmt.Sprintf("%d", account.AttachmentCount),
			formatBytes(account.TotalBytes),
		})
	}
	return rows
}

func labelRows(labels []CacheDashboardLabel) [][]string {
	rows := make([][]string, 0, len(labels))
	for _, label := range labels {
		rows = append(rows, []string{
			depthIndicator(label.CacheDepth),
			firstNonEmpty(label.LabelName, label.LabelID),
			fmt.Sprintf("%d", label.MessageCount),
			fmt.Sprintf("%d", label.BodyCount),
			fmt.Sprintf("%d", label.AttachmentCount),
			formatBytes(label.TotalBytes),
		})
	}
	return rows
}

func ageRows(ages []CacheDashboardAge) [][]string {
	rows := make([][]string, 0, len(ages))
	for _, age := range ages {
		rows = append(rows, []string{
			age.Bucket,
			fmt.Sprintf("%d", age.MessageCount),
			fmt.Sprintf("%d", age.BodyCount),
			fmt.Sprintf("%d", age.AttachmentCount),
			formatBytes(age.TotalBytes),
		})
	}
	return rows
}

func rowRows(counts []CacheDashboardRow) [][]string {
	rows := make([][]string, 0, len(counts))
	for _, count := range counts {
		rows = append(rows, []string{count.Table, fmt.Sprintf("%d", count.Rows)})
	}
	return rows
}

func cacheTable(title string, headings []string, rows [][]string, width int, maxRows int) []string {
	lines := []string{title, fit(strings.Join(headings, "  "), width)}
	if len(rows) == 0 {
		lines = append(lines, fit("No rows", width))
		return lines
	}
	if maxRows > 0 && len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	widths := cacheColumnWidths(headings, rows, width)
	for _, row := range rows {
		parts := make([]string, len(row))
		for i, cell := range row {
			parts[i] = fit(cell, widths[i])
		}
		lines = append(lines, fit(strings.Join(parts, "  "), width))
	}
	return lines
}

func cacheColumnWidths(headings []string, rows [][]string, maxWidth int) []int {
	widths := make([]int, len(headings))
	for i, heading := range headings {
		widths[i] = len([]rune(heading))
	}
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				continue
			}
			if size := len([]rune(cell)); size > widths[i] {
				widths[i] = size
			}
		}
	}
	available := maxWidth - (len(widths)-1)*2
	if available < len(widths) {
		available = len(widths)
	}
	total := 0
	for _, width := range widths {
		total += width
	}
	if total <= available {
		return widths
	}
	over := total - available
	for over > 0 {
		largest := 0
		for i := range widths {
			if widths[i] > widths[largest] {
				largest = i
			}
		}
		if widths[largest] <= 4 {
			break
		}
		widths[largest]--
		over--
	}
	return widths
}

func formatBytes(size int64) string {
	if size < 0 {
		size = 0
	}
	const unit = int64(1024)
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	for _, suffix := range []string{"KB", "MB", "GB"} {
		value /= float64(unit)
		if value < float64(unit) {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f TB", value/float64(unit))
}
