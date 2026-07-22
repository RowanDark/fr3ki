package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeYAML creates a temp file with the given content and returns its path.
func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

// --- LoadConfig tests ---

func TestLoadConfig_PopulatesAllFields(t *testing.T) {
	yaml := `
wordlists:
  - wordlists/common.txt
  - wordlists/extra.txt
threads: 20
output: results.csv
format: csv
rate: 50
cooldown: 5
debug: true
obfuscate: true
verbose: true
proxies: proxies.txt
headers:
  - "X-Token: abc"
  - "X-Foo: bar"
method: POST
filter_codes:
  - 404
  - 403
filter_sizes:
  - 1024
match_sizes:
  - 2048
filter_words:
  - 10
match_words:
  - 50
match_string: "hello"
match_regex: "^OK"
data: "user=FUZZ"
resume: true
recursive: true
recursion_depth: 3
`
	path := writeYAML(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checkStrings(t, "Wordlists", cfg.Wordlists, []string{"wordlists/common.txt", "wordlists/extra.txt"})
	checkInt(t, "Threads", cfg.Threads, 20)
	checkStr(t, "Output", cfg.Output, "results.csv")
	checkStr(t, "Format", cfg.Format, "csv")
	checkInt(t, "Rate", cfg.Rate, 50)
	checkInt(t, "Cooldown", cfg.Cooldown, 5)
	checkBool(t, "Debug", cfg.Debug, true)
	checkBool(t, "Obfuscate", cfg.Obfuscate, true)
	checkBool(t, "Verbose", cfg.Verbose, true)
	checkStr(t, "Proxies", cfg.Proxies, "proxies.txt")
	checkStrings(t, "Headers", cfg.Headers, []string{"X-Token: abc", "X-Foo: bar"})
	checkStr(t, "Method", cfg.Method, "POST")
	checkInts(t, "FilterCodes", cfg.FilterCodes, []int{404, 403})
	checkInts(t, "FilterSizes", cfg.FilterSizes, []int{1024})
	checkInts(t, "MatchSizes", cfg.MatchSizes, []int{2048})
	checkInts(t, "FilterWords", cfg.FilterWords, []int{10})
	checkInts(t, "MatchWords", cfg.MatchWords, []int{50})
	checkStr(t, "MatchString", cfg.MatchString, "hello")
	checkStr(t, "MatchRegex", cfg.MatchRegex, "^OK")
	checkStr(t, "Data", cfg.Data, "user=FUZZ")
	checkBool(t, "Resume", cfg.Resume, true)
	checkBool(t, "Recursive", cfg.Recursive, true)
	checkInt(t, "RecursionDepth", cfg.RecursionDepth, 3)
}

func TestLoadConfig_PartialYAML(t *testing.T) {
	yaml := `
threads: 8
output: out.jsonl
`
	path := writeYAML(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checkInt(t, "Threads", cfg.Threads, 8)
	checkStr(t, "Output", cfg.Output, "out.jsonl")
	// unset fields stay zero
	if cfg.Verbose {
		t.Error("Verbose: want false, got true")
	}
	if len(cfg.Headers) != 0 {
		t.Errorf("Headers: want empty, got %v", cfg.Headers)
	}
}

func TestLoadConfig_MissingFileReturnsZeroConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("want nil error for missing file, got: %v", err)
	}
	if !reflect.DeepEqual(cfg, Config{}) {
		t.Errorf("want zero-value Config, got %+v", cfg)
	}
}

func TestLoadConfig_EmptyFileReturnsZeroConfig(t *testing.T) {
	path := writeYAML(t, "")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(cfg, Config{}) {
		t.Errorf("want zero-value Config, got %+v", cfg)
	}
}

func TestLoadConfig_InvalidYAMLReturnsError(t *testing.T) {
	path := writeYAML(t, "threads: [not an int")
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

// --- Merge tests ---

func TestMerge_FlagsOverrideConfig(t *testing.T) {
	base := Config{
		Threads:        10,
		Output:         "base.jsonl",
		Format:         "jsonl",
		Rate:           0,
		Cooldown:       10,
		Proxies:        "base_proxies.txt",
		Method:         "GET",
		FilterCodes:    []int{404},
		RecursionDepth: 2,
		Wordlists:      []string{"wordlists/common.txt"},
		Headers:        []string{"X-Base: 1"},
	}

	flags := Config{
		Threads:        20,
		Output:         "flags.csv",
		Format:         "csv",
		Rate:           100,
		Cooldown:       30,
		Debug:          true,
		Obfuscate:      true,
		Verbose:        true,
		Proxies:        "flags_proxies.txt",
		Method:         "POST",
		FilterCodes:    []int{404, 403},
		FilterSizes:    []int{512},
		MatchSizes:     []int{1024},
		FilterWords:    []int{5},
		MatchWords:     []int{20},
		MatchString:    "ok",
		MatchRegex:     "^200",
		Data:           "x=FUZZ",
		Resume:         true,
		Recursive:      true,
		RecursionDepth: 5,
		Wordlists:      []string{"wordlists/big.txt"},
		Headers:        []string{"X-Flag: 2"},
	}

	got := base.Merge(flags)

	checkInt(t, "Threads", got.Threads, 20)
	checkStr(t, "Output", got.Output, "flags.csv")
	checkStr(t, "Format", got.Format, "csv")
	checkInt(t, "Rate", got.Rate, 100)
	checkInt(t, "Cooldown", got.Cooldown, 30)
	checkBool(t, "Debug", got.Debug, true)
	checkBool(t, "Obfuscate", got.Obfuscate, true)
	checkBool(t, "Verbose", got.Verbose, true)
	checkStr(t, "Proxies", got.Proxies, "flags_proxies.txt")
	checkStr(t, "Method", got.Method, "POST")
	checkInts(t, "FilterCodes", got.FilterCodes, []int{404, 403})
	checkInts(t, "FilterSizes", got.FilterSizes, []int{512})
	checkInts(t, "MatchSizes", got.MatchSizes, []int{1024})
	checkInts(t, "FilterWords", got.FilterWords, []int{5})
	checkInts(t, "MatchWords", got.MatchWords, []int{20})
	checkStr(t, "MatchString", got.MatchString, "ok")
	checkStr(t, "MatchRegex", got.MatchRegex, "^200")
	checkStr(t, "Data", got.Data, "x=FUZZ")
	checkBool(t, "Resume", got.Resume, true)
	checkBool(t, "Recursive", got.Recursive, true)
	checkInt(t, "RecursionDepth", got.RecursionDepth, 5)
	checkStrings(t, "Wordlists", got.Wordlists, []string{"wordlists/big.txt"})
	checkStrings(t, "Headers", got.Headers, []string{"X-Flag: 2"})
}

func TestMerge_ZeroFlagsKeepConfigValues(t *testing.T) {
	base := Config{
		Threads:        10,
		Output:         "base.jsonl",
		Format:         "jsonl",
		Cooldown:       15,
		Proxies:        "proxies.txt",
		Method:         "GET",
		FilterCodes:    []int{404},
		FilterSizes:    []int{0},
		Wordlists:      []string{"wordlists/common.txt"},
		Headers:        []string{"X-Base: yes"},
		MatchString:    "found",
		MatchRegex:     "pattern",
		Data:           "body=FUZZ",
		RecursionDepth: 3,
		Debug:          true,
		Obfuscate:      true,
		Verbose:        true,
		Resume:         true,
		Recursive:      true,
	}

	got := base.Merge(Config{}) // zero flags — nothing should change

	if !reflect.DeepEqual(got, base) {
		t.Errorf("Merge with zero flags changed the config\ngot:  %+v\nwant: %+v", got, base)
	}
}

func TestMerge_DoesNotMutateReceiver(t *testing.T) {
	base := Config{Threads: 10, Output: "base.jsonl"}
	flags := Config{Threads: 99}
	_ = base.Merge(flags)
	if base.Threads != 10 {
		t.Errorf("Merge mutated receiver: Threads=%d, want 10", base.Threads)
	}
}

func TestMerge_BooleansFalseInFlagsDoNotClearConfigTrue(t *testing.T) {
	base := Config{Debug: true, Obfuscate: true, Verbose: true, Resume: true, Recursive: true}
	flags := Config{} // all booleans false

	got := base.Merge(flags)

	checkBool(t, "Debug", got.Debug, true)
	checkBool(t, "Obfuscate", got.Obfuscate, true)
	checkBool(t, "Verbose", got.Verbose, true)
	checkBool(t, "Resume", got.Resume, true)
	checkBool(t, "Recursive", got.Recursive, true)
}

// --- helpers ---

func checkInt(t *testing.T, name string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", name, got, want)
	}
}

func checkStr(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", name, got, want)
	}
}

func checkBool(t *testing.T, name string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func checkStrings(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: len %d, want %d; got %v", name, len(got), len(want), got)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q", name, i, got[i], want[i])
		}
	}
}

func checkInts(t *testing.T, name string, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: len %d, want %d; got %v", name, len(got), len(want), got)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %d, want %d", name, i, got[i], want[i])
		}
	}
}
