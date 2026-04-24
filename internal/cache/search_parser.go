package cache

import (
	"fmt"
	"strings"
	"unicode"
)

var ErrUnsupportedSearchOperator = fmt.Errorf("unsupported offline search operator")

type OfflineSearchQuery struct {
	Match string
	Args  []any
}

func CompileOfflineSearch(query string) (OfflineSearchQuery, error) {
	tokens, err := searchTokens(query)
	if err != nil {
		return OfflineSearchQuery{}, err
	}
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		field, value, hasField := strings.Cut(token, ":")
		if hasField {
			column, ok := offlineSearchColumn(field)
			if !ok {
				return OfflineSearchQuery{}, fmt.Errorf("%w: %s", ErrUnsupportedSearchOperator, field)
			}
			if strings.TrimSpace(value) == "" {
				return OfflineSearchQuery{}, fmt.Errorf("offline search field %s requires a value", field)
			}
			parts = append(parts, column+":"+quoteFTS5Phrase(value))
			continue
		}
		parts = append(parts, quoteFTS5Phrase(token))
	}
	if len(parts) == 0 {
		return OfflineSearchQuery{}, fmt.Errorf("offline search query is empty")
	}
	match := strings.Join(parts, " AND ")
	return OfflineSearchQuery{Match: match, Args: []any{match}}, nil
}

func offlineSearchColumn(field string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "from":
		return "from_addr", true
	case "to":
		return "to_addrs", true
	case "subject":
		return "subject", true
	default:
		return "", false
	}
}

func searchTokens(query string) ([]string, error) {
	var tokens []string
	var b strings.Builder
	inQuote := false
	var quote rune
	for _, r := range strings.TrimSpace(query) {
		switch {
		case inQuote:
			if r == quote {
				inQuote = false
				continue
			}
			b.WriteRune(r)
		case r == '"' || r == '\'':
			inQuote = true
			quote = r
		case unicode.IsSpace(r):
			if b.Len() > 0 {
				tokens = append(tokens, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("offline search query has an unterminated quote")
	}
	if b.Len() > 0 {
		tokens = append(tokens, b.String())
	}
	return tokens, nil
}

func quoteFTS5Phrase(value string) string {
	escaped := strings.ReplaceAll(strings.TrimSpace(value), `"`, `""`)
	return `"` + escaped + `"`
}
