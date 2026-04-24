package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlaintextQuoteGolden(t *testing.T) {
	input := strings.Join([]string{
		"New reply",
		"",
		"> old line 1",
		"> old line 2",
		"> old line 3",
		"> old line 4",
		"> old line 5",
	}, "\n")

	assertGolden(t, "plaintext_quote.golden", FormatQuotes(input, QuoteOptions{CollapseThreshold: 2}))
}

func TestHTMLNewsletterGolden(t *testing.T) {
	input := `<h1>Weekly Update</h1><p>Hello <strong>team</strong>.</p><ul><li>Ship inbox sync</li><li>Review cache policy</li></ul>`

	got, err := HTMLToText(input, HTMLRenderOptions{URLFootnotes: true})
	if err != nil {
		t.Fatalf("html to text: %v", err)
	}
	assertGolden(t, "html_newsletter.golden", got)
}

func TestHTMLLinksGolden(t *testing.T) {
	input := `<p>Read <a href="https://example.com/post">the post</a> and <a href="https://example.com/docs">docs</a>.</p>`

	got, err := HTMLToText(input, HTMLRenderOptions{URLFootnotes: true})
	if err != nil {
		t.Fatalf("html to text: %v", err)
	}
	assertGolden(t, "html_links.golden", got)
}

func TestHTMLImagesGolden(t *testing.T) {
	input := `<p>Header <img src="https://cdn.example.com/logo.png" alt="Acme logo"></p><p><img src="/banner.jpg"></p>`

	got, err := HTMLToText(input, HTMLRenderOptions{URLFootnotes: true})
	if err != nil {
		t.Fatalf("html to text: %v", err)
	}
	assertGolden(t, "html_images.golden", got)
}

func TestHTMLTablesGolden(t *testing.T) {
	input := `<table><tr><th>Label</th><th>Count</th></tr><tr><td>Inbox</td><td>3</td></tr><tr><td>Sent</td><td>4</td></tr></table>`

	got, err := HTMLToText(input, HTMLRenderOptions{URLFootnotes: true})
	if err != nil {
		t.Fatalf("html to text: %v", err)
	}
	assertGolden(t, "html_table.golden", got)
}

func TestAttachmentsGolden(t *testing.T) {
	lines := FormatAttachments([]Attachment{
		{Name: "plan.pdf", MimeType: "application/pdf", SizeBytes: 1536},
		{Name: "image.png", MimeType: "image/png", SizeBytes: 2048},
	})
	assertGolden(t, "attachments.golden", strings.Join(lines, "\n"))
}

func assertGolden(t *testing.T, name string, got string) {
	t.Helper()

	path := filepath.Join("testdata", name)
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	want := normalizeTextForTest(string(wantBytes))
	got = normalizeTextForTest(got)
	if got != want {
		t.Fatalf("golden mismatch %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
