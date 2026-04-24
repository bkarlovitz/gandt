package gmail

import (
	"encoding/base64"
	"testing"
)

func TestExtractBodyPlainOnly(t *testing.T) {
	body, err := ExtractBody(messageWithPayload(MessagePart{
		MimeType: "text/plain",
		Body:     MessagePartBody{Data: encodedBody("hello plain")},
	}), BodyExtractionOptions{KeepHTML: true})
	if err != nil {
		t.Fatalf("extract body: %v", err)
	}

	preferred, source := body.Preferred()
	if preferred != "hello plain" || source != "plain" {
		t.Fatalf("preferred = %q/%s, want plain body", preferred, source)
	}
	if body.HTML != nil {
		t.Fatalf("html = %v, want nil", body.HTML)
	}
}

func TestExtractBodyHTMLOnly(t *testing.T) {
	body, err := ExtractBody(messageWithPayload(MessagePart{
		MimeType: "text/html; charset=UTF-8",
		Body:     MessagePartBody{Data: encodedBody("<p>hello html</p>")},
	}), BodyExtractionOptions{KeepHTML: true})
	if err != nil {
		t.Fatalf("extract body: %v", err)
	}

	preferred, source := body.Preferred()
	if preferred != "<p>hello html</p>" || source != "html" {
		t.Fatalf("preferred = %q/%s, want html fallback", preferred, source)
	}
	if body.HTML == nil || *body.HTML != "<p>hello html</p>" {
		t.Fatalf("html = %v, want raw html retained", body.HTML)
	}
}

func TestExtractBodyMultipartAlternativePrefersPlain(t *testing.T) {
	body, err := ExtractBody(messageWithPayload(MessagePart{
		MimeType: "multipart/alternative",
		Parts: []MessagePart{
			{PartID: "0", MimeType: "text/plain", Body: MessagePartBody{Data: encodedBody("plain version")}},
			{PartID: "1", MimeType: "text/html", Body: MessagePartBody{Data: encodedBody("<p>html version</p>")}},
		},
	}), BodyExtractionOptions{KeepHTML: true})
	if err != nil {
		t.Fatalf("extract body: %v", err)
	}

	preferred, source := body.Preferred()
	if preferred != "plain version" || source != "plain" {
		t.Fatalf("preferred = %q/%s, want plain version", preferred, source)
	}
	if body.HTML == nil || *body.HTML != "<p>html version</p>" {
		t.Fatalf("html = %v, want retained raw html", body.HTML)
	}
}

func TestExtractBodyNestedMultipart(t *testing.T) {
	body, err := ExtractBody(messageWithPayload(MessagePart{
		MimeType: "multipart/mixed",
		Parts: []MessagePart{
			{
				PartID:   "0",
				MimeType: "multipart/alternative",
				Parts: []MessagePart{
					{PartID: "0.1", MimeType: "text/html", Body: MessagePartBody{Data: encodedBody("<p>nested html</p>")}},
					{PartID: "0.2", MimeType: "text/plain", Body: MessagePartBody{Data: encodedBody("nested plain")}},
				},
			},
		},
	}), BodyExtractionOptions{KeepHTML: true})
	if err != nil {
		t.Fatalf("extract body: %v", err)
	}

	preferred, source := body.Preferred()
	if preferred != "nested plain" || source != "plain" {
		t.Fatalf("preferred = %q/%s, want nested plain", preferred, source)
	}
}

func TestExtractBodyPreservesAttachmentMetadata(t *testing.T) {
	body, err := ExtractBody(messageWithPayload(MessagePart{
		MimeType: "multipart/mixed",
		Parts: []MessagePart{
			{PartID: "0", MimeType: "text/plain", Body: MessagePartBody{Data: encodedBody("see attached")}},
			{
				PartID:   "1",
				MimeType: "application/pdf",
				Filename: "report.pdf",
				Body:     MessagePartBody{AttachmentID: "att-1", Size: 2048},
			},
		},
	}), BodyExtractionOptions{KeepHTML: true})
	if err != nil {
		t.Fatalf("extract body: %v", err)
	}

	if len(body.Attachments) != 1 {
		t.Fatalf("attachments = %#v, want one attachment", body.Attachments)
	}
	attachment := body.Attachments[0]
	if attachment.PartID != "1" || attachment.Filename != "report.pdf" || attachment.MimeType != "application/pdf" || attachment.AttachmentID != "att-1" || attachment.Size != 2048 {
		t.Fatalf("attachment = %#v, want preserved metadata", attachment)
	}
}

func TestExtractBodyLeavesRawHTMLNilWhenNotKept(t *testing.T) {
	body, err := ExtractBody(messageWithPayload(MessagePart{
		MimeType: "text/html",
		Body:     MessagePartBody{Data: encodedBody("<p>transient html</p>")},
	}), BodyExtractionOptions{KeepHTML: false})
	if err != nil {
		t.Fatalf("extract body: %v", err)
	}

	preferred, source := body.Preferred()
	if preferred != "<p>transient html</p>" || source != "html" {
		t.Fatalf("preferred = %q/%s, want html fallback", preferred, source)
	}
	if body.HTML != nil {
		t.Fatalf("html = %v, want nil when raw HTML is not kept", body.HTML)
	}
}

func messageWithPayload(part MessagePart) Message {
	return Message{ID: "message-1", Payload: &part}
}

func encodedBody(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}
