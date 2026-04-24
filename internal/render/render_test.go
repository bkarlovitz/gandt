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

func TestHTMLRawGolden(t *testing.T) {
	input := `<section><h1>Sale</h1><p>Open in Gmail for the full newsletter.</p></section>`

	got, err := HTMLBody(input, HTMLRenderModeRawHTML, HTMLRenderOptions{URLFootnotes: true})
	if err != nil {
		t.Fatalf("raw html: %v", err)
	}
	assertGolden(t, "html_raw.golden", got)
}

func TestHTMLGlamourGolden(t *testing.T) {
	input := `<h1>Sale</h1><p>Visit <a href="https://example.com/store">the store</a>.</p>`

	got, err := HTMLBody(input, HTMLRenderModeGlamour, HTMLRenderOptions{URLFootnotes: true, Width: 72})
	if err != nil {
		t.Fatalf("glamour html: %v", err)
	}
	assertGolden(t, "html_glamour.golden", got)
}

func TestHTMLVisibleTextFallbackForSparseNewsletter(t *testing.T) {
	input := `<html><head><style>.x{display:none}</style></head><body><div>` +
		strings.Repeat(`<span data-tracking="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"></span>`, 30) +
		`<p>Primary offer</p><p>Open the account dashboard.</p></div></body></html>`

	got, err := HTMLToText(input, HTMLRenderOptions{URLFootnotes: true})
	if err != nil {
		t.Fatalf("html to text: %v", err)
	}
	if !strings.Contains(got, "Primary offer") || !strings.Contains(got, "Open the account dashboard.") {
		t.Fatalf("fallback text missing visible content:\n%s", got)
	}
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
