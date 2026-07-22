package fuzzer

import (
	"log"
	"regexp"
	"strings"
)

// InterestingCodes mirrors INTERESTING_CODES from fr3ki.py. These status codes
// trigger a file save even when debug mode is off.
var InterestingCodes = map[int]bool{
	200: true, 201: true, 202: true, 204: true,
	301: true, 302: true, 307: true, 308: true,
	401: true, 403: true, 405: true, 500: true,
}

// IsInteresting reports whether statusCode is in InterestingCodes.
func IsInteresting(statusCode int) bool {
	return InterestingCodes[statusCode]
}

// ResponseInfo holds the extracted fields needed for filter/match evaluation.
type ResponseInfo struct {
	StatusCode int
	Size       int    // byte length of body
	Words      int    // whitespace-delimited token count, matching Python's len(text.split())
	Body       string // decoded body text
}

// NewResponseInfo builds a ResponseInfo from a status code and raw body bytes,
// computing Size and Words automatically.
func NewResponseInfo(statusCode int, body []byte) ResponseInfo {
	text := string(body)
	return ResponseInfo{
		StatusCode: statusCode,
		Size:       len(body),
		Words:      len(strings.Fields(text)),
		Body:       text,
	}
}

// ShouldSave reports whether the response should be written to the output file.
// It mirrors the Python save decision: debug OR interesting code OR body > 500 B.
func ShouldSave(r ResponseInfo, debug bool) bool {
	return debug || IsInteresting(r.StatusCode) || r.Size > 500
}

// Filter holds pre-compiled, validated state for response filtering. Construct
// via NewFilter; the zero value passes everything.
type Filter struct {
	filterSizes []int
	matchSizes  []int
	filterWords []int
	matchWords  []int
	matchString string
	matchRegex  *regexp.Regexp // nil when unset or pattern was invalid
}

// NewFilter creates a Filter from the provided parameters. If matchRegex is
// non-empty but syntactically invalid, an error is logged and that check is
// disabled rather than panicking.
func NewFilter(filterSizes, matchSizes, filterWords, matchWords []int, matchString, matchRegex string) *Filter {
	f := &Filter{
		filterSizes: filterSizes,
		matchSizes:  matchSizes,
		filterWords: filterWords,
		matchWords:  matchWords,
		matchString: matchString,
	}
	if matchRegex != "" {
		rx, err := regexp.Compile(matchRegex)
		if err != nil {
			log.Printf("filter: invalid match_regex %q: %v (check skipped)", matchRegex, err)
		} else {
			f.matchRegex = rx
		}
	}
	return f
}

// ShouldKeep returns true if the response passes all configured filters and
// satisfies all configured matchers. The evaluation order mirrors fr3ki.py:
// filter_sizes → match_sizes → filter_words → match_words → match_string → match_regex.
func (f *Filter) ShouldKeep(r ResponseInfo) bool {
	// 1. filter_sizes: suppress if size is in the filter list.
	for _, s := range f.filterSizes {
		if r.Size == s {
			return false
		}
	}
	// 2. match_sizes: suppress if non-empty and size is not in the match list.
	if len(f.matchSizes) > 0 {
		matched := false
		for _, s := range f.matchSizes {
			if r.Size == s {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	// 3. filter_words: suppress if word count is in the filter list.
	for _, w := range f.filterWords {
		if r.Words == w {
			return false
		}
	}
	// 4. match_words: suppress if non-empty and word count is not in the match list.
	if len(f.matchWords) > 0 {
		matched := false
		for _, w := range f.matchWords {
			if r.Words == w {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	// 5. match_string: suppress if non-empty and body does not contain the string.
	if f.matchString != "" && !strings.Contains(r.Body, f.matchString) {
		return false
	}
	// 6. match_regex: suppress if compiled and body does not match.
	if f.matchRegex != nil && !f.matchRegex.MatchString(r.Body) {
		return false
	}
	return true
}
