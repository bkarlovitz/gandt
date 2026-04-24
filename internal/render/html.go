package render

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	html2text "jaytaylor.com/html2text"
)

type HTMLRenderOptions struct {
	URLFootnotes bool
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
