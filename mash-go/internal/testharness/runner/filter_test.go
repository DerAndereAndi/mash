package runner

import (
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name, pattern string
		want          bool
	}{
		{"TC-001", "*", true},
		{"TC-001", "", true},
		{"TC-001", "TC-001", true},
		{"TC-001", "TC-002", false},
		{"TC-DISC-001", "TC-DISC*", true},
		{"TC-DISC-001", "*DISC*", true},
		{"TC-DISC-001", "*001", true},
		{"TC-DISC-001", "TC-COMM*", false},
		{"discovery-basic", "*basic", true},
		{"discovery-basic", "discovery*", true},
	}
	for _, tt := range tests {
		t.Run(tt.name+"_"+tt.pattern, func(t *testing.T) {
			got := matchPattern(tt.name, tt.pattern)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.name, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestFilterByPattern_CommaSeparated(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-CLOSE-001", Name: "close basic"},
		{ID: "TC-CLOSE-002", Name: "close ack"},
		{ID: "TC-KEEPALIVE-001", Name: "keepalive basic"},
		{ID: "TC-KEEPALIVE-002", Name: "keepalive timeout"},
		{ID: "TC-PASE-001", Name: "pase basic"},
		{ID: "TC-ERR-001", Name: "error invalid endpoint"},
	}

	// Comma-separated patterns should match any sub-pattern.
	filtered := filterByPattern(cases, "TC-CLOSE*,TC-KEEP*")
	if len(filtered) != 4 {
		t.Fatalf("expected 4 matches, got %d: %v", len(filtered), ids(filtered))
	}
	assertContainsID(t, filtered, "TC-CLOSE-001")
	assertContainsID(t, filtered, "TC-CLOSE-002")
	assertContainsID(t, filtered, "TC-KEEPALIVE-001")
	assertContainsID(t, filtered, "TC-KEEPALIVE-002")
}

func TestFilterByPattern_SinglePattern(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-CLOSE-001"},
		{ID: "TC-CLOSE-002"},
		{ID: "TC-PASE-001"},
	}

	// Single pattern should work exactly as before.
	filtered := filterByPattern(cases, "TC-CLOSE*")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(filtered))
	}
}

func TestFilterByPattern_EmptySegments(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-CLOSE-001"},
		{ID: "TC-PASE-001"},
	}

	// Empty segments between commas should be ignored.
	filtered := filterByPattern(cases, "TC-CLOSE*,,TC-PASE*")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(filtered))
	}
}

func TestFilterByPattern_WhitespaceHandling(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-CLOSE-001"},
		{ID: "TC-PASE-001"},
		{ID: "TC-ERR-001"},
	}

	// Whitespace around segments should be trimmed.
	filtered := filterByPattern(cases, "TC-CLOSE* , TC-PASE* ")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(filtered))
	}
	assertContainsID(t, filtered, "TC-CLOSE-001")
	assertContainsID(t, filtered, "TC-PASE-001")
}

func TestFilterByPattern_ExactMatch(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-CLOSE-001"},
		{ID: "TC-CLOSE-002"},
	}

	// Exact ID in comma list.
	filtered := filterByPattern(cases, "TC-CLOSE-001,TC-CLOSE-002")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(filtered))
	}
}

func TestFilterByPattern_MatchAll(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-CLOSE-001"},
		{ID: "TC-PASE-001"},
	}

	// Wildcard should match all.
	filtered := filterByPattern(cases, "*")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(filtered))
	}

	// Empty string should match all.
	filtered = filterByPattern(cases, "")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matches for empty pattern, got %d", len(filtered))
	}
}

func TestFilterByPattern_NoDuplicates(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-CLOSE-001", Name: "close basic"},
	}

	// Pattern matches both ID and Name -- should not duplicate.
	filtered := filterByPattern(cases, "TC-CLOSE*,*basic")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 match (no duplicates), got %d", len(filtered))
	}
}

func TestFilterByTags(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-001", Tags: []string{"slow", "connection"}},
		{ID: "TC-002", Tags: []string{"fast"}},
		{ID: "TC-003", Tags: []string{"slow", "reaper"}},
		{ID: "TC-004"}, // no tags
	}

	// Single tag
	filtered := filterByTags(cases, "slow")
	if len(filtered) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(filtered), ids(filtered))
	}
	assertContainsID(t, filtered, "TC-001")
	assertContainsID(t, filtered, "TC-003")

	// Multiple tags (comma-separated OR)
	filtered = filterByTags(cases, "fast,reaper")
	if len(filtered) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(filtered), ids(filtered))
	}
	assertContainsID(t, filtered, "TC-002")
	assertContainsID(t, filtered, "TC-003")

	// No match
	filtered = filterByTags(cases, "nonexistent")
	if len(filtered) != 0 {
		t.Fatalf("expected 0, got %d", len(filtered))
	}
}

func TestFilterByTags_EmptyFilter(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-001", Tags: []string{"slow"}},
		{ID: "TC-002"},
	}

	// Empty string returns all
	filtered := filterByTags(cases, "")
	if len(filtered) != 2 {
		t.Fatalf("expected 2, got %d", len(filtered))
	}
}

func TestFilterByExcludeTags(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-001", Tags: []string{"slow", "connection"}},
		{ID: "TC-002", Tags: []string{"fast"}},
		{ID: "TC-003", Tags: []string{"slow", "reaper"}},
		{ID: "TC-004"}, // no tags
	}

	// Exclude slow
	filtered := filterByExcludeTags(cases, "slow")
	if len(filtered) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(filtered), ids(filtered))
	}
	assertContainsID(t, filtered, "TC-002")
	assertContainsID(t, filtered, "TC-004")

	// Exclude multiple (comma-separated OR)
	filtered = filterByExcludeTags(cases, "slow,fast")
	if len(filtered) != 1 {
		t.Fatalf("expected 1, got %d: %v", len(filtered), ids(filtered))
	}
	assertContainsID(t, filtered, "TC-004")

	// Exclude nonexistent -- keeps all
	filtered = filterByExcludeTags(cases, "nonexistent")
	if len(filtered) != 4 {
		t.Fatalf("expected 4, got %d", len(filtered))
	}

	// Empty exclude -- keeps all
	filtered = filterByExcludeTags(cases, "")
	if len(filtered) != 4 {
		t.Fatalf("expected 4, got %d", len(filtered))
	}
}

// helpers

func ids(cases []*loader.TestCase) []string {
	out := make([]string, len(cases))
	for i, c := range cases {
		out[i] = c.ID
	}
	return out
}

func assertContainsID(t *testing.T, cases []*loader.TestCase, id string) {
	t.Helper()
	for _, c := range cases {
		if c.ID == id {
			return
		}
	}
	t.Errorf("expected to find %s in results %v", id, ids(cases))
}
