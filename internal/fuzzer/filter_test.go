package fuzzer

import (
	"bytes"
	"log"
	"os"
	"testing"
)

// --- NewResponseInfo ---

func TestNewResponseInfo_Fields(t *testing.T) {
	body := []byte("hello world foo")
	r := NewResponseInfo(200, body)
	if r.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", r.StatusCode)
	}
	if r.Size != 15 {
		t.Errorf("Size: got %d, want 15", r.Size)
	}
	if r.Words != 3 {
		t.Errorf("Words: got %d, want 3", r.Words)
	}
	if r.Body != "hello world foo" {
		t.Errorf("Body: got %q, want %q", r.Body, "hello world foo")
	}
}

func TestNewResponseInfo_Empty(t *testing.T) {
	r := NewResponseInfo(404, nil)
	if r.Size != 0 {
		t.Errorf("Size: got %d, want 0", r.Size)
	}
	if r.Words != 0 {
		t.Errorf("Words: got %d, want 0", r.Words)
	}
}

func TestNewResponseInfo_WhitespaceOnlyBody(t *testing.T) {
	r := NewResponseInfo(200, []byte("   \n\t  "))
	if r.Words != 0 {
		t.Errorf("Words: got %d, want 0 for whitespace-only body", r.Words)
	}
}

// --- IsInteresting ---

func TestIsInteresting_AllKnownCodes(t *testing.T) {
	interesting := []int{200, 201, 202, 204, 301, 302, 307, 308, 401, 403, 405, 500}
	for _, code := range interesting {
		if !IsInteresting(code) {
			t.Errorf("IsInteresting(%d) = false, want true", code)
		}
	}
}

func TestIsInteresting_BoringCodes(t *testing.T) {
	boring := []int{400, 404, 410, 429, 503, 0, 999}
	for _, code := range boring {
		if IsInteresting(code) {
			t.Errorf("IsInteresting(%d) = true, want false", code)
		}
	}
}

// --- ShouldSave ---

func TestShouldSave_Debug_AlwaysSaves(t *testing.T) {
	r := ResponseInfo{StatusCode: 404, Size: 0}
	if !ShouldSave(r, true) {
		t.Error("debug=true should always save regardless of code or size")
	}
}

func TestShouldSave_InterestingCode_Saves(t *testing.T) {
	r := ResponseInfo{StatusCode: 200, Size: 0}
	if !ShouldSave(r, false) {
		t.Error("interesting code should trigger save without debug")
	}
}

func TestShouldSave_LargeBody_Saves(t *testing.T) {
	r := ResponseInfo{StatusCode: 404, Size: 501}
	if !ShouldSave(r, false) {
		t.Error("size > 500 should trigger save without debug")
	}
}

func TestShouldSave_Exactly500_NotSaved(t *testing.T) {
	// Boundary: >500 saves, ==500 does not.
	r := ResponseInfo{StatusCode: 404, Size: 500}
	if ShouldSave(r, false) {
		t.Error("size == 500 should not trigger the size-based save")
	}
}

func TestShouldSave_BoringSmallResponse_NotSaved(t *testing.T) {
	r := ResponseInfo{StatusCode: 404, Size: 100}
	if ShouldSave(r, false) {
		t.Error("boring code + small body + debug=false should not save")
	}
}

// --- ShouldKeep: filter_sizes ---

func TestShouldKeep_FilterSize_Excluded(t *testing.T) {
	f := NewFilter([]int{100}, nil, nil, nil, "", "")
	r := ResponseInfo{Size: 100, Words: 5, Body: "hello world foo bar baz"}
	if f.ShouldKeep(r) {
		t.Error("response whose size is in filter_sizes should be excluded")
	}
}

func TestShouldKeep_FilterSize_NotExcluded(t *testing.T) {
	f := NewFilter([]int{100}, nil, nil, nil, "", "")
	r := ResponseInfo{Size: 200, Words: 1, Body: "body"}
	if !f.ShouldKeep(r) {
		t.Error("response with different size should pass filter_sizes")
	}
}

func TestShouldKeep_FilterSize_MultipleValues(t *testing.T) {
	f := NewFilter([]int{100, 200, 300}, nil, nil, nil, "", "")
	excluded := []int{100, 200, 300}
	for _, sz := range excluded {
		r := ResponseInfo{Size: sz, Body: "x"}
		if f.ShouldKeep(r) {
			t.Errorf("size %d should be excluded by filter_sizes", sz)
		}
	}
	r := ResponseInfo{Size: 400, Body: "x"}
	if !f.ShouldKeep(r) {
		t.Error("size 400 is not in filter_sizes and should pass")
	}
}

// --- ShouldKeep: match_sizes ---

func TestShouldKeep_MatchSize_Included(t *testing.T) {
	f := NewFilter(nil, []int{100}, nil, nil, "", "")
	r := ResponseInfo{Size: 100, Body: "x"}
	if !f.ShouldKeep(r) {
		t.Error("response with matching size should be kept")
	}
}

func TestShouldKeep_MatchSize_Excluded(t *testing.T) {
	f := NewFilter(nil, []int{100}, nil, nil, "", "")
	r := ResponseInfo{Size: 200, Body: "x"}
	if f.ShouldKeep(r) {
		t.Error("response not in match_sizes should be excluded when match_sizes is set")
	}
}

func TestShouldKeep_MatchSize_EmptyMeansNoRestriction(t *testing.T) {
	f := NewFilter(nil, nil, nil, nil, "", "")
	r := ResponseInfo{Size: 99999, Body: "x"}
	if !f.ShouldKeep(r) {
		t.Error("empty match_sizes should impose no restriction")
	}
}

func TestShouldKeep_MatchSize_MultipleAllowed(t *testing.T) {
	f := NewFilter(nil, []int{100, 200}, nil, nil, "", "")
	if !f.ShouldKeep(ResponseInfo{Size: 100, Body: "x"}) {
		t.Error("size 100 should be kept")
	}
	if !f.ShouldKeep(ResponseInfo{Size: 200, Body: "x"}) {
		t.Error("size 200 should be kept")
	}
	if f.ShouldKeep(ResponseInfo{Size: 150, Body: "x"}) {
		t.Error("size 150 not in match_sizes should be excluded")
	}
}

// --- ShouldKeep: filter_words ---

func TestShouldKeep_FilterWords_Excluded(t *testing.T) {
	f := NewFilter(nil, nil, []int{3}, nil, "", "")
	r := ResponseInfo{Words: 3, Body: "hello world foo"}
	if f.ShouldKeep(r) {
		t.Error("response with word count in filter_words should be excluded")
	}
}

func TestShouldKeep_FilterWords_NotExcluded(t *testing.T) {
	f := NewFilter(nil, nil, []int{3}, nil, "", "")
	r := ResponseInfo{Words: 2, Body: "hello world"}
	if !f.ShouldKeep(r) {
		t.Error("response with different word count should pass filter_words")
	}
}

func TestShouldKeep_FilterWords_MultipleValues(t *testing.T) {
	f := NewFilter(nil, nil, []int{1, 2, 5}, nil, "", "")
	if f.ShouldKeep(ResponseInfo{Words: 1}) {
		t.Error("word count 1 should be excluded")
	}
	if f.ShouldKeep(ResponseInfo{Words: 5}) {
		t.Error("word count 5 should be excluded")
	}
	if !f.ShouldKeep(ResponseInfo{Words: 3}) {
		t.Error("word count 3 should pass")
	}
}

// --- ShouldKeep: match_words ---

func TestShouldKeep_MatchWords_Included(t *testing.T) {
	f := NewFilter(nil, nil, nil, []int{3}, "", "")
	r := ResponseInfo{Words: 3, Body: "hello world foo"}
	if !f.ShouldKeep(r) {
		t.Error("response with matching word count should be kept")
	}
}

func TestShouldKeep_MatchWords_Excluded(t *testing.T) {
	f := NewFilter(nil, nil, nil, []int{3}, "", "")
	r := ResponseInfo{Words: 2, Body: "hello world"}
	if f.ShouldKeep(r) {
		t.Error("response not in match_words should be excluded when match_words is set")
	}
}

func TestShouldKeep_MatchWords_EmptyMeansNoRestriction(t *testing.T) {
	f := NewFilter(nil, nil, nil, nil, "", "")
	r := ResponseInfo{Words: 99, Body: "x"}
	if !f.ShouldKeep(r) {
		t.Error("empty match_words should impose no restriction")
	}
}

// --- ShouldKeep: match_string ---

func TestShouldKeep_MatchString_Found(t *testing.T) {
	f := NewFilter(nil, nil, nil, nil, "secret", "")
	r := ResponseInfo{Body: "contains secret inside"}
	if !f.ShouldKeep(r) {
		t.Error("response containing match_string should be kept")
	}
}

func TestShouldKeep_MatchString_NotFound(t *testing.T) {
	f := NewFilter(nil, nil, nil, nil, "secret", "")
	r := ResponseInfo{Body: "nothing here"}
	if f.ShouldKeep(r) {
		t.Error("response not containing match_string should be excluded")
	}
}

func TestShouldKeep_MatchString_Empty(t *testing.T) {
	f := NewFilter(nil, nil, nil, nil, "", "")
	r := ResponseInfo{Body: "anything"}
	if !f.ShouldKeep(r) {
		t.Error("empty match_string should impose no restriction")
	}
}

func TestShouldKeep_MatchString_CaseSensitive(t *testing.T) {
	f := NewFilter(nil, nil, nil, nil, "Secret", "")
	r := ResponseInfo{Body: "contains secret (lowercase)"}
	if f.ShouldKeep(r) {
		t.Error("match_string check should be case-sensitive")
	}
}

// --- ShouldKeep: match_regex ---

func TestShouldKeep_MatchRegex_Matches(t *testing.T) {
	f := NewFilter(nil, nil, nil, nil, "", `\d{3}`)
	r := ResponseInfo{Body: "status 404 not found"}
	if !f.ShouldKeep(r) {
		t.Error("response matching regex should be kept")
	}
}

func TestShouldKeep_MatchRegex_NoMatch(t *testing.T) {
	f := NewFilter(nil, nil, nil, nil, "", `\d{3}`)
	r := ResponseInfo{Body: "hello world"}
	if f.ShouldKeep(r) {
		t.Error("response not matching regex should be excluded")
	}
}

func TestShouldKeep_MatchRegex_Empty(t *testing.T) {
	f := NewFilter(nil, nil, nil, nil, "", "")
	r := ResponseInfo{Body: "anything"}
	if !f.ShouldKeep(r) {
		t.Error("empty match_regex should impose no restriction")
	}
}

// TestShouldKeep_MatchRegex_Invalid verifies that an invalid regex pattern
// produces a logged error and skips the check rather than panicking.
func TestShouldKeep_MatchRegex_Invalid(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	f := NewFilter(nil, nil, nil, nil, "", `[invalid`)
	// Check is skipped: response should still pass (not excluded).
	r := ResponseInfo{Body: "any body text"}
	if !f.ShouldKeep(r) {
		t.Error("invalid regex should skip the check; response should not be excluded")
	}
	if buf.Len() == 0 {
		t.Error("invalid regex should produce a log error message")
	}
}

// TestShouldKeep_MatchRegex_InvalidNoPanic ensures NewFilter with an invalid
// regex does not panic.
func TestShouldKeep_MatchRegex_InvalidNoPanic(t *testing.T) {
	log.SetOutput(bytes.NewBuffer(nil))
	defer log.SetOutput(os.Stderr)

	defer func() {
		if rec := recover(); rec != nil {
			t.Errorf("NewFilter panicked with invalid regex: %v", rec)
		}
	}()
	NewFilter(nil, nil, nil, nil, "", `(unclosed`)
}

// --- ShouldKeep: filter order and combinations ---

// TestShouldKeep_FilterRunsBeforeMatch ensures filter_sizes is evaluated before
// match_sizes (a size that is both filtered and matched should be excluded).
func TestShouldKeep_FilterRunsBeforeMatch(t *testing.T) {
	f := NewFilter([]int{100}, []int{100}, nil, nil, "", "")
	r := ResponseInfo{Size: 100, Body: "x"}
	if f.ShouldKeep(r) {
		t.Error("filter_sizes runs before match_sizes: size 100 should be excluded")
	}
}

// TestShouldKeep_SizePassStringFail: passes match_sizes but fails match_string.
func TestShouldKeep_SizePassStringFail(t *testing.T) {
	f := NewFilter(nil, []int{100}, nil, nil, "needle", "")
	r := ResponseInfo{Size: 100, Body: "haystack has no needle word here... wait, needle"}
	// Body does contain "needle" so let's use one that doesn't.
	r.Body = "haystack without target"
	if f.ShouldKeep(r) {
		t.Error("should be excluded: passes size match but fails string match")
	}
}

// TestShouldKeep_WordFilterBlocksStringMatch: word count excluded even though
// match_string is satisfied.
func TestShouldKeep_WordFilterBlocksStringMatch(t *testing.T) {
	f := NewFilter(nil, nil, []int{3}, nil, "hello", "")
	r := ResponseInfo{Words: 3, Body: "hello world foo"}
	if f.ShouldKeep(r) {
		t.Error("filter_words should exclude before match_string is evaluated")
	}
}

// TestShouldKeep_StringPresentRegexFails: match_string satisfied but regex not.
func TestShouldKeep_StringPresentRegexFails(t *testing.T) {
	f := NewFilter(nil, nil, nil, nil, "hello", `\d+`)
	r := ResponseInfo{Body: "hello world (no digits)"}
	if f.ShouldKeep(r) {
		t.Error("should be excluded: string matches but regex does not")
	}
}

// TestShouldKeep_AllFiltersPass: every filter and matcher is set and satisfied.
func TestShouldKeep_AllFiltersPass(t *testing.T) {
	f := NewFilter(
		[]int{50},       // filter_sizes: exclude size 50
		[]int{100},      // match_sizes: require size 100
		[]int{10},       // filter_words: exclude 10 words
		[]int{3},        // match_words: require 3 words
		"hello",         // match_string
		`hello\s+world`, // match_regex
	)
	r := ResponseInfo{Size: 100, Words: 3, Body: "hello world foo"}
	if !f.ShouldKeep(r) {
		t.Error("response satisfying all filters and matchers should be kept")
	}
}

// TestShouldKeep_NoFilters: zero-value filter passes everything.
func TestShouldKeep_NoFilters(t *testing.T) {
	f := NewFilter(nil, nil, nil, nil, "", "")
	responses := []ResponseInfo{
		{StatusCode: 200, Size: 0, Words: 0, Body: ""},
		{StatusCode: 404, Size: 1000, Words: 200, Body: "big response"},
		{StatusCode: 500, Size: 42, Words: 7, Body: "error page content here"},
	}
	for _, r := range responses {
		if !f.ShouldKeep(r) {
			t.Errorf("zero-value filter should keep every response, failed for %+v", r)
		}
	}
}
