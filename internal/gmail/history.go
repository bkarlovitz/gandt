package gmail

import (
	"context"
	"fmt"
	"strconv"

	gmailapi "google.golang.org/api/gmail/v1"
)

func (c *Client) ListHistory(ctx context.Context, opts ListHistoryOptions) (HistoryPage, error) {
	startHistoryID, err := parseHistoryID(opts.StartHistoryID)
	if err != nil {
		return HistoryPage{}, err
	}

	var response *gmailapi.ListHistoryResponse
	if err := c.withRetry(ctx, "list gmail history", func() error {
		call := c.service.Users.History.List("me").StartHistoryId(startHistoryID).Context(ctx)
		if opts.PageToken != "" {
			call.PageToken(opts.PageToken)
		}
		if opts.MaxResults > 0 {
			call.MaxResults(opts.MaxResults)
		}
		if opts.LabelID != "" {
			call.LabelId(opts.LabelID)
		}
		if len(opts.HistoryTypes) > 0 {
			call.HistoryTypes(opts.HistoryTypes...)
		}
		var err error
		response, err = call.Do()
		return err
	}); err != nil {
		return HistoryPage{}, err
	}
	return convertHistoryPage(response), nil
}

func parseHistoryID(value string) (uint64, error) {
	if value == "" {
		return 0, fmt.Errorf("gmail history id is required")
	}
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse gmail history id %q: %w", value, err)
	}
	return id, nil
}

func convertHistoryPage(response *gmailapi.ListHistoryResponse) HistoryPage {
	if response == nil {
		return HistoryPage{}
	}
	records := make([]HistoryRecord, 0, len(response.History))
	for _, record := range response.History {
		if record == nil {
			continue
		}
		records = append(records, convertHistoryRecord(record))
	}
	return HistoryPage{
		Records:       records,
		NextPageToken: response.NextPageToken,
		HistoryID:     historyIDString(response.HistoryId),
	}
}

func convertHistoryRecord(record *gmailapi.History) HistoryRecord {
	out := HistoryRecord{ID: historyIDString(record.Id)}
	for _, change := range record.MessagesAdded {
		if change == nil {
			continue
		}
		out.MessagesAdded = append(out.MessagesAdded, HistoryMessageChange{Message: convertHistoryMessageRef(change.Message)})
	}
	for _, change := range record.MessagesDeleted {
		if change == nil {
			continue
		}
		out.MessagesDeleted = append(out.MessagesDeleted, HistoryMessageChange{Message: convertHistoryMessageRef(change.Message)})
	}
	for _, change := range record.LabelsAdded {
		if change == nil {
			continue
		}
		out.LabelsAdded = append(out.LabelsAdded, HistoryLabelChange{
			Message:  convertHistoryMessageRef(change.Message),
			LabelIDs: append([]string{}, change.LabelIds...),
		})
	}
	for _, change := range record.LabelsRemoved {
		if change == nil {
			continue
		}
		out.LabelsRemoved = append(out.LabelsRemoved, HistoryLabelChange{
			Message:  convertHistoryMessageRef(change.Message),
			LabelIDs: append([]string{}, change.LabelIds...),
		})
	}
	return out
}

func convertHistoryMessageRef(message *gmailapi.Message) MessageRef {
	if message == nil {
		return MessageRef{}
	}
	return MessageRef{ID: message.Id, ThreadID: message.ThreadId}
}
