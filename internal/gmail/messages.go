package gmail

import (
	"context"
	"fmt"
	"time"

	gmailapi "google.golang.org/api/gmail/v1"
)

type MessageFormat string

const (
	MessageFormatMetadata MessageFormat = "metadata"
	MessageFormatFull     MessageFormat = "full"
)

type MessageReader interface {
	ListMessages(context.Context, ListMessagesOptions) (ListMessagesPage, error)
	ListHistory(context.Context, ListHistoryOptions) (HistoryPage, error)
	GetMessageMetadata(context.Context, string, ...string) (Message, error)
	GetMessageFull(context.Context, string) (Message, error)
	BatchGetMessageMetadata(context.Context, []string, ...string) ([]Message, error)
	GetThread(context.Context, string, MessageFormat, ...string) (Thread, error)
}

type ListMessagesOptions struct {
	LabelIDs         []string
	Query            string
	PageToken        string
	MaxResults       int64
	IncludeSpamTrash bool
}

type ListMessagesPage struct {
	Messages           []MessageRef
	NextPageToken      string
	ResultSizeEstimate int
}

type ListHistoryOptions struct {
	StartHistoryID string
	PageToken      string
	MaxResults     int64
	LabelID        string
	HistoryTypes   []string
}

type HistoryPage struct {
	Records       []HistoryRecord
	NextPageToken string
	HistoryID     string
}

type HistoryRecord struct {
	ID              string
	MessagesAdded   []HistoryMessageChange
	MessagesDeleted []HistoryMessageChange
	LabelsAdded     []HistoryLabelChange
	LabelsRemoved   []HistoryLabelChange
}

type HistoryMessageChange struct {
	Message MessageRef
}

type HistoryLabelChange struct {
	Message  MessageRef
	LabelIDs []string
}

type MessageRef struct {
	ID       string
	ThreadID string
}

type Message struct {
	ID           string
	ThreadID     string
	HistoryID    string
	LabelIDs     []string
	Snippet      string
	SizeEstimate int
	InternalDate time.Time
	Raw          string
	Headers      []MessageHeader
	Payload      *MessagePart
}

type MessageHeader struct {
	Name  string
	Value string
}

type MessagePart struct {
	PartID   string
	MimeType string
	Filename string
	Headers  []MessageHeader
	Body     MessagePartBody
	Parts    []MessagePart
}

type MessagePartBody struct {
	Data         string
	Size         int
	AttachmentID string
}

type Thread struct {
	ID        string
	HistoryID string
	Snippet   string
	Messages  []Message
}

func (c *Client) ListMessages(ctx context.Context, opts ListMessagesOptions) (ListMessagesPage, error) {
	var response *gmailapi.ListMessagesResponse
	if err := c.withRetry(ctx, "list gmail messages", func() error {
		call := c.service.Users.Messages.List("me").Context(ctx)
		if len(opts.LabelIDs) > 0 {
			call.LabelIds(opts.LabelIDs...)
		}
		if opts.Query != "" {
			call.Q(opts.Query)
		}
		if opts.PageToken != "" {
			call.PageToken(opts.PageToken)
		}
		if opts.MaxResults > 0 {
			call.MaxResults(opts.MaxResults)
		}
		if opts.IncludeSpamTrash {
			call.IncludeSpamTrash(true)
		}
		var err error
		response, err = call.Do()
		return err
	}); err != nil {
		return ListMessagesPage{}, err
	}

	messages := make([]MessageRef, 0, len(response.Messages))
	for _, message := range response.Messages {
		if message == nil {
			continue
		}
		messages = append(messages, MessageRef{
			ID:       message.Id,
			ThreadID: message.ThreadId,
		})
	}
	return ListMessagesPage{
		Messages:           messages,
		NextPageToken:      response.NextPageToken,
		ResultSizeEstimate: int(response.ResultSizeEstimate),
	}, nil
}

func (c *Client) GetMessageMetadata(ctx context.Context, id string, headers ...string) (Message, error) {
	return c.getMessage(ctx, id, MessageFormatMetadata, headers...)
}

func (c *Client) GetMessageFull(ctx context.Context, id string) (Message, error) {
	return c.getMessage(ctx, id, MessageFormatFull)
}

func (c *Client) BatchGetMessageMetadata(ctx context.Context, ids []string, headers ...string) ([]Message, error) {
	messages := make([]Message, 0, len(ids))
	for _, id := range ids {
		message, err := c.GetMessageMetadata(ctx, id, headers...)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func (c *Client) GetThread(ctx context.Context, id string, format MessageFormat, headers ...string) (Thread, error) {
	if format == "" {
		format = MessageFormatFull
	}
	var response *gmailapi.Thread
	if err := c.withRetry(ctx, "get gmail thread", func() error {
		call := c.service.Users.Threads.Get("me", id).Format(string(format)).Context(ctx)
		if format == MessageFormatMetadata && len(headers) > 0 {
			call.MetadataHeaders(headers...)
		}
		var err error
		response, err = call.Do()
		return err
	}); err != nil {
		return Thread{}, err
	}
	return convertThread(response), nil
}

func (c *Client) getMessage(ctx context.Context, id string, format MessageFormat, headers ...string) (Message, error) {
	if format == "" {
		format = MessageFormatMetadata
	}
	var response *gmailapi.Message
	if err := c.withRetry(ctx, fmt.Sprintf("get gmail message %s", id), func() error {
		call := c.service.Users.Messages.Get("me", id).Format(string(format)).Context(ctx)
		if format == MessageFormatMetadata && len(headers) > 0 {
			call.MetadataHeaders(headers...)
		}
		var err error
		response, err = call.Do()
		return err
	}); err != nil {
		return Message{}, err
	}
	return convertMessage(response), nil
}

func convertThread(thread *gmailapi.Thread) Thread {
	if thread == nil {
		return Thread{}
	}
	messages := make([]Message, 0, len(thread.Messages))
	for _, message := range thread.Messages {
		messages = append(messages, convertMessage(message))
	}
	return Thread{
		ID:        thread.Id,
		HistoryID: historyIDString(thread.HistoryId),
		Snippet:   thread.Snippet,
		Messages:  messages,
	}
}

func convertMessage(message *gmailapi.Message) Message {
	if message == nil {
		return Message{}
	}
	return Message{
		ID:           message.Id,
		ThreadID:     message.ThreadId,
		HistoryID:    historyIDString(message.HistoryId),
		LabelIDs:     append([]string{}, message.LabelIds...),
		Snippet:      message.Snippet,
		SizeEstimate: int(message.SizeEstimate),
		InternalDate: unixMilli(message.InternalDate),
		Raw:          message.Raw,
		Headers:      convertHeaders(payloadHeaders(message.Payload)),
		Payload:      convertPartPtr(message.Payload),
	}
}

func payloadHeaders(payload *gmailapi.MessagePart) []*gmailapi.MessagePartHeader {
	if payload == nil {
		return nil
	}
	return payload.Headers
}

func convertPartPtr(part *gmailapi.MessagePart) *MessagePart {
	if part == nil {
		return nil
	}
	converted := convertPart(part)
	return &converted
}

func convertPart(part *gmailapi.MessagePart) MessagePart {
	converted := MessagePart{
		PartID:   part.PartId,
		MimeType: part.MimeType,
		Filename: part.Filename,
		Headers:  convertHeaders(part.Headers),
	}
	if part.Body != nil {
		converted.Body = MessagePartBody{
			Data:         part.Body.Data,
			Size:         int(part.Body.Size),
			AttachmentID: part.Body.AttachmentId,
		}
	}
	if len(part.Parts) > 0 {
		converted.Parts = make([]MessagePart, 0, len(part.Parts))
		for _, child := range part.Parts {
			if child == nil {
				continue
			}
			converted.Parts = append(converted.Parts, convertPart(child))
		}
	}
	return converted
}

func convertHeaders(headers []*gmailapi.MessagePartHeader) []MessageHeader {
	out := make([]MessageHeader, 0, len(headers))
	for _, header := range headers {
		if header == nil {
			continue
		}
		out = append(out, MessageHeader{Name: header.Name, Value: header.Value})
	}
	return out
}

func historyIDString(id uint64) string {
	if id == 0 {
		return ""
	}
	return fmt.Sprintf("%d", id)
}

func unixMilli(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}
