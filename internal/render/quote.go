package render

import (
	"fmt"
	"strings"
)

type QuoteOptions struct {
	CollapseThreshold int
	ShowQuotes        bool
}

func FormatQuotes(input string, opts QuoteOptions) string {
	if opts.ShowQuotes {
		return strings.TrimSpace(input)
	}
	threshold := opts.CollapseThreshold
	if threshold <= 0 {
		threshold = 4
	}

	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		if !isQuoteLine(lines[i]) {
			out = append(out, lines[i])
			i++
			continue
		}

		start := i
		for i < len(lines) && isQuoteLine(lines[i]) {
			i++
		}
		block := lines[start:i]
		if len(block) > threshold {
			out = append(out, block[:threshold]...)
			out = append(out, fmt.Sprintf("[quoted text collapsed; %d lines hidden]", len(block)-threshold))
			continue
		}
		out = append(out, block...)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func isQuoteLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), ">")
}
