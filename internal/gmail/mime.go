package gmail

import (
	"encoding/base64"
	"fmt"
	"mime"
	"strings"
)

type BodyExtractionOptions struct {
	KeepHTML bool
}

type ExtractedBody struct {
	Plain        *string
	HTML         *string
	FallbackHTML *string
	Attachments  []MIMEAttachment
}

type MIMEAttachment struct {
	PartID       string
	Filename     string
	MimeType     string
	Size         int
	AttachmentID string
}

func ExtractBody(message Message, opts BodyExtractionOptions) (ExtractedBody, error) {
	if message.Payload == nil {
		return ExtractedBody{}, nil
	}

	state := bodyExtractionState{keepHTML: opts.KeepHTML}
	if err := state.walk(*message.Payload); err != nil {
		return ExtractedBody{}, err
	}
	return state.body(), nil
}

func (body ExtractedBody) Preferred() (string, string) {
	if body.Plain != nil {
		return *body.Plain, "plain"
	}
	if body.FallbackHTML != nil {
		return *body.FallbackHTML, "html"
	}
	return "", ""
}

type bodyExtractionState struct {
	keepHTML    bool
	plainParts  []string
	htmlParts   []string
	attachments []MIMEAttachment
}

func (s *bodyExtractionState) walk(part MessagePart) error {
	mimeType := normalizedMimeType(part.MimeType)
	if isAttachment(part) {
		s.attachments = append(s.attachments, MIMEAttachment{
			PartID:       part.PartID,
			Filename:     part.Filename,
			MimeType:     mimeType,
			Size:         part.Body.Size,
			AttachmentID: part.Body.AttachmentID,
		})
		return nil
	}

	switch mimeType {
	case "text/plain":
		decoded, err := decodeGmailBody(part.Body.Data)
		if err != nil {
			return fmt.Errorf("decode text/plain part %s: %w", part.PartID, err)
		}
		if decoded != "" {
			s.plainParts = append(s.plainParts, decoded)
		}
	case "text/html":
		decoded, err := decodeGmailBody(part.Body.Data)
		if err != nil {
			return fmt.Errorf("decode text/html part %s: %w", part.PartID, err)
		}
		if decoded != "" {
			s.htmlParts = append(s.htmlParts, decoded)
		}
	}

	for _, child := range part.Parts {
		if err := s.walk(child); err != nil {
			return err
		}
	}
	return nil
}

func (s bodyExtractionState) body() ExtractedBody {
	out := ExtractedBody{
		Attachments: append([]MIMEAttachment{}, s.attachments...),
	}
	if len(s.plainParts) > 0 {
		plain := strings.Join(s.plainParts, "\n\n")
		out.Plain = &plain
	}
	if len(s.htmlParts) > 0 {
		html := strings.Join(s.htmlParts, "\n\n")
		out.FallbackHTML = &html
		if s.keepHTML {
			out.HTML = &html
		}
	}
	return out
}

func decodeGmailBody(data string) (string, error) {
	if data == "" {
		return "", nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(data)
	}
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func normalizedMimeType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return value
	}
	return mediaType
}

func isAttachment(part MessagePart) bool {
	return part.Filename != "" || part.Body.AttachmentID != ""
}
