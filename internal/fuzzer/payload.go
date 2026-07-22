package fuzzer

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// ObfuscatePayload returns all obfuscation transform variants of word, deduplicated.
// Mirrors Python's obfuscate_payload():
//
//  1. Identity (original word)
//  2. URL-encode  (urllib.parse.quote with default safe='/')
//  3. Base64
//  4. Hex per char  (%XX for each Unicode code point, lowercase)
//  5. Slash/colon escape  (%2F / %3A)
//  6. Unicode escape  (\uXXXX per code point, lowercase)
//  7. Double URL-encode
func ObfuscatePayload(word string) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(s string) {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}

	add(word)
	add(urlQuote(word))
	add(base64.StdEncoding.EncodeToString([]byte(word)))
	add(hexPerChar(word))
	add(strings.NewReplacer("/", "%2F", ":", "%3A").Replace(word))
	add(unicodeEscape(word))
	add(urlQuote(urlQuote(word)))

	return out
}

// ExpandWordlist applies ObfuscatePayload to every word and returns the deduplicated union
// across all words, matching Python's per-wordlist expansion before product generation.
func ExpandWordlist(words []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, w := range words {
		for _, v := range ObfuscatePayload(w) {
			if _, ok := seen[v]; !ok {
				seen[v] = struct{}{}
				out = append(out, v)
			}
		}
	}
	return out
}

// CartesianProduct returns the Cartesian product of wordlists.
// A single wordlist produces single-element payload sets, matching Python's
// `[(w,) for w in words]`; multiple wordlists use `itertools.product(*all_words)`.
func CartesianProduct(wordlists [][]string) [][]string {
	if len(wordlists) == 0 {
		return nil
	}
	result := [][]string{{}}
	for _, wl := range wordlists {
		next := make([][]string, 0, len(result)*len(wl))
		for _, existing := range result {
			for _, word := range wl {
				combo := make([]string, len(existing)+1)
				copy(combo, existing)
				combo[len(existing)] = word
				next = append(next, combo)
			}
		}
		result = next
	}
	return result
}

// BuildPayloadSets builds the final payload set matrix from loaded wordlists.
// If obfuscate is true, each wordlist is first expanded with ExpandWordlist.
// A non-empty warning is returned when obfuscation is combined with multiple wordlists
// and the estimated combination count exceeds 100,000.
func BuildPayloadSets(wordlists [][]string, obfuscate bool) ([][]string, string) {
	var warn string
	if obfuscate && len(wordlists) > 1 {
		if est := estimateCombinations(wordlists, true); est > 100_000 {
			warn = fmt.Sprintf(
				"combinatorial explosion: obfuscation across %d wordlists yields ~%d combinations (>100,000); this may be slow",
				len(wordlists), est,
			)
		}
	}

	expanded := make([][]string, len(wordlists))
	for i, wl := range wordlists {
		if obfuscate {
			expanded[i] = ExpandWordlist(wl)
		} else {
			expanded[i] = wl
		}
	}
	return CartesianProduct(expanded), warn
}

// estimateCombinations returns an upper-bound on the total payload count.
// When obfuscate is true, each word is assumed to produce at most 7 variants.
func estimateCombinations(wordlists [][]string, obfuscate bool) int64 {
	total := int64(1)
	for _, wl := range wordlists {
		n := int64(len(wl))
		if obfuscate {
			n *= 7
		}
		total *= n
	}
	return total
}

// urlQuote percent-encodes s keeping unreserved chars (A–Z a–z 0–9 - _ . ~) and '/'
// unchanged — matching Python urllib.parse.quote(s) with default safe='/'.
// Non-ASCII bytes are each encoded as %XX with uppercase hex.
func urlQuote(s string) string {
	var buf strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if isUnreservedByte(b) || b == '/' {
			buf.WriteByte(b)
		} else {
			fmt.Fprintf(&buf, "%%%02X", b)
		}
	}
	return buf.String()
}

func isUnreservedByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') ||
		(b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == '~'
}

// hexPerChar encodes each Unicode code point of s as %XX (lowercase hex),
// matching Python's ''.join(['%{:02x}'.format(ord(c)) for c in word]).
func hexPerChar(s string) string {
	var buf strings.Builder
	for _, c := range s {
		fmt.Fprintf(&buf, "%%%02x", c)
	}
	return buf.String()
}

// unicodeEscape encodes each Unicode code point of s as \uXXXX (lowercase hex),
// matching Python's ''.join(['\\u{:04x}'.format(ord(c)) for c in word]).
func unicodeEscape(s string) string {
	var buf strings.Builder
	for _, c := range s {
		fmt.Fprintf(&buf, "\\u%04x", c)
	}
	return buf.String()
}
