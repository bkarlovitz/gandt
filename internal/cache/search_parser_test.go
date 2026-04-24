package cache

import (
	"errors"
	"testing"
)

func TestCompileOfflineSearchFieldsAndTerms(t *testing.T) {
	query, err := CompileOfflineSearch(`from:ada@example.com to:team@example.com subject:"Quarterly plan" migration`)
	if err != nil {
		t.Fatalf("compile offline search: %v", err)
	}
	want := `from_addr:"ada@example.com" AND to_addrs:"team@example.com" AND subject:"Quarterly plan" AND "migration"`
	if query.Match != want {
		t.Fatalf("match = %q, want %q", query.Match, want)
	}
	if len(query.Args) != 1 || query.Args[0] != query.Match {
		t.Fatalf("args = %#v, want single match parameter", query.Args)
	}
}

func TestCompileOfflineSearchRejectsUnsupportedOperator(t *testing.T) {
	_, err := CompileOfflineSearch("has:attachment")
	if !errors.Is(err, ErrUnsupportedSearchOperator) {
		t.Fatalf("error = %v, want unsupported operator", err)
	}
}

func TestCompileOfflineSearchQuotesInjectionLikeInput(t *testing.T) {
	query, err := CompileOfflineSearch(`subject:"plan" OR messages_fts MATCH '*'`)
	if err != nil {
		t.Fatalf("compile offline search: %v", err)
	}
	want := `subject:"plan" AND "OR" AND "messages_fts" AND "MATCH" AND "*"`
	if query.Match != want {
		t.Fatalf("match = %q, want %q", query.Match, want)
	}
}

func TestCompileOfflineSearchReportsBadInput(t *testing.T) {
	if _, err := CompileOfflineSearch("   "); err == nil {
		t.Fatal("expected empty query error")
	}
	if _, err := CompileOfflineSearch(`subject:"unterminated`); err == nil {
		t.Fatal("expected unterminated quote error")
	}
	if _, err := CompileOfflineSearch("from:"); err == nil {
		t.Fatal("expected missing field value error")
	}
}
