package fuzzer

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadWordlist reads a wordlist file and returns its non-blank, whitespace-trimmed lines.
func LoadWordlist(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open wordlist %q: %w", path, err)
	}
	defer f.Close()

	var words []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			words = append(words, line)
		}
	}
	return words, sc.Err()
}

// AssignFuzzKeys returns the FUZZ keyword(s) for n wordlists.
// One wordlist gets the bare "FUZZ" key; multiple wordlists get "FUZZ1", "FUZZ2", etc.,
// matching the Python assignment logic.
func AssignFuzzKeys(n int) []string {
	if n == 1 {
		return []string{"FUZZ"}
	}
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("FUZZ%d", i+1)
	}
	return keys
}

// SubstitutePayload replaces each FUZZ key in s with the corresponding payload value,
// matching Python's `url = url.replace(key, payload)` loop over (fuzz_keys, payload_set).
func SubstitutePayload(s string, keys []string, payloadSet []string) string {
	for i, key := range keys {
		if i >= len(payloadSet) {
			break
		}
		s = strings.ReplaceAll(s, key, payloadSet[i])
	}
	return s
}
