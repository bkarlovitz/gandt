package ui

import (
	"fmt"
	"strconv"
	"strings"
)

type CachePurgeRequest struct {
	Account       string
	LabelID       string
	OlderThanDays int
	From          string
	DryRun        bool
}

type CachePurgePreview struct {
	Request         CachePurgeRequest
	MessageCount    int
	BodyCount       int
	AttachmentCount int
	EstimatedBytes  int64
}

type CachePurgeStore interface {
	PlanCachePurge(CachePurgeRequest) (CachePurgePreview, error)
}

type CachePurgeStoreFunc func(CachePurgeRequest) (CachePurgePreview, error)

func (fn CachePurgeStoreFunc) PlanCachePurge(request CachePurgeRequest) (CachePurgePreview, error) {
	return fn(request)
}

func parseCachePurgeRequest(fields []string) (CachePurgeRequest, error) {
	request := CachePurgeRequest{}
	for i := 1; i < len(fields); i++ {
		switch fields[i] {
		case "--dry-run":
			request.DryRun = true
		case "--account":
			if i+1 >= len(fields) {
				return CachePurgeRequest{}, fmt.Errorf("--account requires a value")
			}
			i++
			request.Account = fields[i]
		case "--label":
			if i+1 >= len(fields) {
				return CachePurgeRequest{}, fmt.Errorf("--label requires a value")
			}
			i++
			request.LabelID = fields[i]
		case "--older-than":
			if i+1 >= len(fields) {
				return CachePurgeRequest{}, fmt.Errorf("--older-than requires a value")
			}
			i++
			days, err := parseDays(fields[i])
			if err != nil {
				return CachePurgeRequest{}, err
			}
			request.OlderThanDays = days
		case "--from":
			if i+1 >= len(fields) {
				return CachePurgeRequest{}, fmt.Errorf("--from requires a value")
			}
			i++
			request.From = fields[i]
		default:
			return CachePurgeRequest{}, fmt.Errorf("unknown cache purge flag: %s", fields[i])
		}
	}
	return request, nil
}

func parseDays(value string) (int, error) {
	value = strings.TrimSuffix(strings.TrimSpace(value), "d")
	days, err := strconv.Atoi(value)
	if err != nil || days <= 0 {
		return 0, fmt.Errorf("--older-than must be positive days")
	}
	return days, nil
}

func cachePurgePreviewSummary(preview CachePurgePreview) string {
	prefix := "cache purge preview"
	if preview.Request.DryRun {
		prefix = "cache purge dry run"
	}
	return fmt.Sprintf("%s: %d messages, %d bodies, %d attachments, %s",
		prefix,
		preview.MessageCount,
		preview.BodyCount,
		preview.AttachmentCount,
		formatBytes(preview.EstimatedBytes),
	)
}
