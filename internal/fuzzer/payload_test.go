package fuzzer

import (
	"fmt"
	"testing"
)

// containsString reports whether needle appears in haystack.
func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestObfuscatePayload_NoDuplicates(t *testing.T) {
	for _, word := range []string{"admin", "root", "hello world", "http://example.com/path"} {
		variants := ObfuscatePayload(word)
		seen := make(map[string]bool)
		for _, v := range variants {
			if seen[v] {
				t.Errorf("ObfuscatePayload(%q): duplicate variant %q", word, v)
			}
			seen[v] = true
		}
	}
}

func TestObfuscatePayload_ContainsOriginal(t *testing.T) {
	word := "admin"
	if !containsString(ObfuscatePayload(word), word) {
		t.Errorf("ObfuscatePayload(%q): original not in result", word)
	}
}

func TestObfuscatePayload_URLEncode(t *testing.T) {
	// space (0x20) is not unreserved → %20 (uppercase hex from urlQuote)
	if !containsString(ObfuscatePayload("hello world"), "hello%20world") {
		t.Errorf("missing URL-encoded variant")
	}
}

func TestObfuscatePayload_Base64(t *testing.T) {
	// base64("hello world") = "aGVsbG8gd29ybGQ="
	if !containsString(ObfuscatePayload("hello world"), "aGVsbG8gd29ybGQ=") {
		t.Errorf("missing base64 variant")
	}
}

func TestObfuscatePayload_HexPerChar(t *testing.T) {
	// "hi" → %68%69  (lowercase, matching Python's {:02x})
	if !containsString(ObfuscatePayload("hi"), "%68%69") {
		t.Errorf("missing hex-per-char variant")
	}
}

func TestObfuscatePayload_SlashColonEscape(t *testing.T) {
	// "http://example.com/path" → : becomes %3A, / becomes %2F
	word := "http://example.com/path"
	want := "http%3A%2F%2Fexample.com%2Fpath"
	if !containsString(ObfuscatePayload(word), want) {
		t.Errorf("ObfuscatePayload(%q): missing slash/colon variant %q, got %v", word, want, ObfuscatePayload(word))
	}
}

func TestObfuscatePayload_UnicodeEscape(t *testing.T) {
	// "hi" → hi  (lowercase, matching Python's {:04x})
	want := "\\u0068\\u0069"
	if !containsString(ObfuscatePayload("hi"), want) {
		t.Errorf("missing unicode-escape variant %q, got %v", want, ObfuscatePayload("hi"))
	}
}

func TestObfuscatePayload_DoubleURLEncode(t *testing.T) {
	// "hello world" → "hello%20world" → "hello%2520world"
	if !containsString(ObfuscatePayload("hello world"), "hello%2520world") {
		t.Errorf("missing double-URL-encode variant")
	}
}

func TestObfuscatePayload_AllUnreserved_Deduplicates(t *testing.T) {
	// "admin" — all chars are URL-safe, so url-encode, slash/colon, and double-encode
	// all collapse to "admin", leaving only 4 distinct variants.
	variants := ObfuscatePayload("admin")
	want := map[string]bool{
		"admin":                             true, // original / url-encode / slash-colon / double-encode
		"YWRtaW4=":                          true, // base64
		"%61%64%6d%69%6e":                   true, // hex per char
		"\\u0061\\u0064\\u006d\\u0069\\u006e": true, // unicode escape
	}
	if len(variants) != len(want) {
		t.Errorf("ObfuscatePayload(%q): got %d variants, want %d: %v", "admin", len(variants), len(want), variants)
	}
	for _, v := range variants {
		if !want[v] {
			t.Errorf("ObfuscatePayload(%q): unexpected variant %q", "admin", v)
		}
	}
}

func TestCartesianProduct_SingleWordlist(t *testing.T) {
	words := []string{"a", "b", "c"}
	sets := CartesianProduct([][]string{words})

	if len(sets) != 3 {
		t.Fatalf("expected 3 payload sets, got %d", len(sets))
	}
	for i, s := range sets {
		if len(s) != 1 {
			t.Errorf("set[%d]: want 1 element, got %d", i, len(s))
			continue
		}
		if s[0] != words[i] {
			t.Errorf("set[%d][0] = %q, want %q", i, s[0], words[i])
		}
	}
}

func TestCartesianProduct_TwoWordlists(t *testing.T) {
	sets := CartesianProduct([][]string{{"a", "b"}, {"x", "y"}})

	if len(sets) != 4 {
		t.Fatalf("expected 4 payload sets (2×2), got %d", len(sets))
	}
	want := [][]string{{"a", "x"}, {"a", "y"}, {"b", "x"}, {"b", "y"}}
	for i, got := range sets {
		if !equalSlices(got, want[i]) {
			t.Errorf("set[%d] = %v, want %v", i, got, want[i])
		}
	}
}

func TestCartesianProduct_ThreeWordlists(t *testing.T) {
	sets := CartesianProduct([][]string{{"a", "b"}, {"x", "y"}, {"1", "2"}})
	if len(sets) != 8 {
		t.Fatalf("expected 8 payload sets (2×2×2), got %d", len(sets))
	}
	if !equalSlices(sets[0], []string{"a", "x", "1"}) {
		t.Errorf("sets[0] = %v, want [a x 1]", sets[0])
	}
	if !equalSlices(sets[7], []string{"b", "y", "2"}) {
		t.Errorf("sets[7] = %v, want [b y 2]", sets[7])
	}
}

func TestCartesianProduct_Empty(t *testing.T) {
	if got := CartesianProduct(nil); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func TestBuildPayloadSets_NoObfuscate_SingleWordlist(t *testing.T) {
	sets, warn := BuildPayloadSets([][]string{{"admin", "root"}}, false)
	if warn != "" {
		t.Errorf("unexpected warning: %q", warn)
	}
	if len(sets) != 2 {
		t.Fatalf("expected 2 payload sets, got %d", len(sets))
	}
	if sets[0][0] != "admin" || sets[1][0] != "root" {
		t.Errorf("unexpected payload sets: %v", sets)
	}
}

func TestBuildPayloadSets_NoObfuscate_TwoWordlists(t *testing.T) {
	sets, warn := BuildPayloadSets([][]string{{"a", "b"}, {"x", "y"}}, false)
	if warn != "" {
		t.Errorf("unexpected warning: %q", warn)
	}
	if len(sets) != 4 {
		t.Fatalf("expected 4 payload sets, got %d", len(sets))
	}
}

func TestBuildPayloadSets_WithObfuscate_SingleWordlist(t *testing.T) {
	// Single wordlist — no Cartesian product, no warning regardless of size.
	sets, warn := BuildPayloadSets([][]string{{"admin"}}, true)
	if warn != "" {
		t.Errorf("unexpected warning for single wordlist: %q", warn)
	}
	// "admin" expands to 4 distinct variants (url-encode / slash-colon / double-encode collapse).
	if len(sets) < 2 {
		t.Errorf("expected multiple payload sets after obfuscation, got %d", len(sets))
	}
	for i, s := range sets {
		if len(s) != 1 {
			t.Errorf("set[%d]: want 1 element, got %d", i, len(s))
		}
	}
}

func TestBuildPayloadSets_CombinationWarning(t *testing.T) {
	// 2 wordlists × 50 words → estimate 50×7 × 50×7 = 122,500 > 100,000 → warning fires.
	wl1 := make([]string, 50)
	wl2 := make([]string, 50)
	for i := range wl1 {
		wl1[i] = fmt.Sprintf("word%d", i)
		wl2[i] = fmt.Sprintf("item%d", i)
	}
	_, warn := BuildPayloadSets([][]string{wl1, wl2}, true)
	if warn == "" {
		t.Error("expected combinatorial explosion warning, got none")
	}
}

func TestBuildPayloadSets_NoWarning_BelowThreshold(t *testing.T) {
	// 2 wordlists × 3 words → estimate 3×7 × 3×7 = 441 < 100,000 → no warning.
	_, warn := BuildPayloadSets([][]string{{"a", "b", "c"}, {"x", "y", "z"}}, true)
	if warn != "" {
		t.Errorf("unexpected warning below threshold: %q", warn)
	}
}

func TestBuildPayloadSets_NoWarning_SingleWordlistLarge(t *testing.T) {
	// Large single wordlist with obfuscation should never warn (no Cartesian product).
	wl := make([]string, 1000)
	for i := range wl {
		wl[i] = fmt.Sprintf("w%d", i)
	}
	_, warn := BuildPayloadSets([][]string{wl}, true)
	if warn != "" {
		t.Errorf("unexpected warning for single wordlist: %q", warn)
	}
}

func TestExpandWordlist_NoDuplicates(t *testing.T) {
	// Duplicate words in input must not produce duplicate variants in output.
	result := ExpandWordlist([]string{"admin", "admin"})
	seen := make(map[string]bool)
	for _, v := range result {
		if seen[v] {
			t.Errorf("ExpandWordlist: duplicate %q in output", v)
		}
		seen[v] = true
	}
}

func TestExpandWordlist_MoreThanInput(t *testing.T) {
	// "hello world" has transforms that differ from the original.
	result := ExpandWordlist([]string{"hello world"})
	if len(result) <= 1 {
		t.Errorf("expected multiple variants, got %d", len(result))
	}
}
