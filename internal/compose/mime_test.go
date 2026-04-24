package compose

import (
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
)

func TestAssembleMIMEPlainText(t *testing.T) {
	raw, err := AssembleMIME(Draft{
		Headers: Headers{
			ActiveAccountID: "acct-1",
			SendAs:          NewAddress("me@example.com"),
			To:              []Address{NewAddress("you@example.com")},
			Subject:         "Hello",
		},
		Body: BodySource{PlainText: "Hi there"},
	})
	if err != nil {
		t.Fatalf("assemble mime: %v", err)
	}

	message, err := ParseMIMEMessage(raw)
	if err != nil {
		t.Fatalf("parse mime: %v", err)
	}
	if message.Header.Get("From") != "me@example.com" || message.Header.Get("To") != "you@example.com" || message.Header.Get("Subject") != "Hello" {
		t.Fatalf("headers = %#v", message.Header)
	}
	body, _ := io.ReadAll(message.Body)
	if got := strings.TrimSpace(string(body)); got != "Hi there" {
		t.Fatalf("body = %q, want plain text", got)
	}
}

func TestAssembleMIMEAlternativeAndAttachment(t *testing.T) {
	raw, err := AssembleMIME(Draft{
		Headers: Headers{
			ActiveAccountID: "acct-1",
			SendAs:          NewAddress("me@example.com"),
			To:              []Address{NewAddress("you@example.com")},
			Cc:              []Address{NewAddress("copy@example.com")},
			Subject:         "Résumé",
		},
		Body: BodySource{
			PlainText: "Plain body",
			HTML:      "<p>Plain body</p>",
		},
		Attachments: []Attachment{{
			Filename:  "report.txt",
			MimeType:  "text/plain",
			SizeBytes: 6,
			Data:      []byte("report"),
		}},
	})
	if err != nil {
		t.Fatalf("assemble mime: %v", err)
	}

	message, err := ParseMIMEMessage(raw)
	if err != nil {
		t.Fatalf("parse mime: %v", err)
	}
	if message.Header.Get("Subject") != "=?utf-8?q?R=C3=A9sum=C3=A9?=" {
		t.Fatalf("subject = %q, want encoded utf-8", message.Header.Get("Subject"))
	}
	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	if mediaType != "multipart/mixed" || params["boundary"] != mixedBoundary {
		t.Fatalf("content-type = %s %#v", mediaType, params)
	}

	reader := multipart.NewReader(message.Body, params["boundary"])
	first, err := reader.NextPart()
	if err != nil {
		t.Fatalf("first part: %v", err)
	}
	altType, altParams, err := mime.ParseMediaType(first.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse alternative content-type: %v", err)
	}
	if altType != "multipart/alternative" || altParams["boundary"] != alternativeBoundary {
		t.Fatalf("alternative content-type = %s %#v", altType, altParams)
	}

	attachment, err := reader.NextPart()
	if err != nil {
		t.Fatalf("attachment part: %v", err)
	}
	if attachment.FileName() != "report.txt" || attachment.Header.Get("Content-Transfer-Encoding") != "base64" {
		t.Fatalf("attachment headers = %#v", attachment.Header)
	}
	encoded, _ := io.ReadAll(attachment)
	if strings.TrimSpace(string(encoded)) != "cmVwb3J0" {
		t.Fatalf("attachment body = %q, want base64 report", string(encoded))
	}
}
