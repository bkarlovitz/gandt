package render

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/charmbracelet/glamour"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	html2text "jaytaylor.com/html2text"
)

type HTMLRenderMode string

const (
	HTMLRenderModePlaintext HTMLRenderMode = "plaintext"
	HTMLRenderModeHTMLText  HTMLRenderMode = "html2text"
	HTMLRenderModeRawHTML   HTMLRenderMode = "raw_html"
	HTMLRenderModeGlamour   HTMLRenderMode = "glamour"
)

type HTMLRenderOptions struct {
	URLFootnotes bool
	Width        int
}

func HTMLToText(input string, opts HTMLRenderOptions) (string, error) {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	links := decorateHTML(doc, opts)
	text, err := html2text.FromHTMLNode(doc, html2text.Options{
		PrettyTables: true,
		OmitLinks:    true,
	})
	if err != nil {
		return "", fmt.Errorf("render html: %w", err)
	}
	text = strings.TrimSpace(text)
	if shouldUseVisibleTextFallback(input, text) {
		text = visibleText(doc)
	}

	if opts.URLFootnotes && len(links) > 0 {
		var b strings.Builder
		b.WriteString(text)
		b.WriteString("\n\n")
		for i, link := range links {
			b.WriteString(fmt.Sprintf("[^%d]: %s", i+1, link))
			if i < len(links)-1 {
				b.WriteByte('\n')
			}
		}
		text = b.String()
	}
	return text, nil
}

func HTMLBody(input string, mode HTMLRenderMode, opts HTMLRenderOptions) (string, error) {
	switch mode {
	case HTMLRenderModeRawHTML:
		return strings.TrimSpace(strings.ReplaceAll(input, "\r\n", "\n")), nil
	case HTMLRenderModeGlamour:
		text, err := HTMLToText(input, opts)
		if err != nil {
			return "", err
		}
		width := opts.Width
		if width <= 0 {
			width = 100
		}
		renderer, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("notty"),
			glamour.WithWordWrap(width),
			glamour.WithPreservedNewLines(),
		)
		if err != nil {
			return "", fmt.Errorf("create glamour renderer: %w", err)
		}
		rendered, err := renderer.Render(text)
		if err != nil {
			return "", fmt.Errorf("render glamour: %w", err)
		}
		return strings.TrimSpace(rendered), nil
	case HTMLRenderModePlaintext, HTMLRenderModeHTMLText, "":
		return HTMLToText(input, opts)
	default:
		return "", fmt.Errorf("unsupported html render mode %q", mode)
	}
}

func decorateHTML(node *html.Node, opts HTMLRenderOptions) []string {
	links := []string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Img {
			replaceImageWithPlaceholder(n)
			return
		}
		if opts.URLFootnotes && n.Type == html.ElementNode && n.DataAtom == atom.A {
			href := strings.TrimSpace(attr(n, "href"))
			if href != "" {
				links = append(links, href)
				n.AppendChild(&html.Node{Type: html.TextNode, Data: fmt.Sprintf(" [^%d]", len(links))})
			}
		}
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			walk(child)
			child = next
		}
	}
	walk(node)
	return links
}

func shouldUseVisibleTextFallback(input string, text string) bool {
	htmlBytes := len(strings.TrimSpace(input))
	textBytes := len(strings.TrimSpace(text))
	return htmlBytes > 200 && textBytes > 0 && textBytes*10 < htmlBytes
}

func visibleText(node *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Script, atom.Style, atom.Head, atom.Meta, atom.Link:
				return
			case atom.Br, atom.P, atom.Div, atom.Tr, atom.Li, atom.H1, atom.H2, atom.H3:
				writeVisibleBreak(&b)
			}
		}
		if n.Type == html.TextNode {
			text := strings.Join(strings.Fields(n.Data), " ")
			if text != "" {
				if b.Len() > 0 && !strings.HasSuffix(b.String(), " ") && !strings.HasSuffix(b.String(), "\n") {
					b.WriteByte(' ')
				}
				b.WriteString(text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.P, atom.Div, atom.Tr, atom.Li, atom.H1, atom.H2, atom.H3:
				writeVisibleBreak(&b)
			}
		}
	}
	walk(node)
	return strings.TrimSpace(compactBlankLines(b.String()))
}

func writeVisibleBreak(w io.StringWriter) {
	_, _ = w.WriteString("\n")
}

func compactBlankLines(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if !blank && len(out) > 0 {
				out = append(out, "")
			}
			blank = true
			continue
		}
		out = append(out, line)
		blank = false
	}
	return strings.Join(out, "\n")
}

func replaceImageWithPlaceholder(n *html.Node) {
	placeholder := imagePlaceholder(n)
	replacement := &html.Node{Type: html.TextNode, Data: placeholder}
	if n.Parent == nil {
		*n = *replacement
		return
	}
	n.Parent.InsertBefore(replacement, n)
	n.Parent.RemoveChild(n)
}

func imagePlaceholder(n *html.Node) string {
	alt := strings.TrimSpace(attr(n, "alt"))
	if alt != "" {
		return "[image: " + alt + "]"
	}
	src := strings.TrimSpace(attr(n, "src"))
	if src != "" {
		name := path.Base(src)
		if name != "." && name != "/" {
			return "[image: " + name + "]"
		}
	}
	return "[image]"
}

func attr(n *html.Node, name string) string {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, name) {
			return attr.Val
		}
	}
	return ""
}

func normalizeTextForTest(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func htmlFragment(nodes ...*html.Node) string {
	var b bytes.Buffer
	for _, node := range nodes {
		_ = html.Render(&b, node)
	}
	return b.String()
}
