package fuzzer

import (
	"fmt"
	"os"
	"testing"
)

// writeWordlist writes lines to a temp file and returns its path.
func writeWordlist(t *testing.T, lines []string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "wl-*.txt")
	if err != nil {
		t.Fatalf("create temp wordlist: %v", err)
	}
	defer f.Close()
	for _, line := range lines {
		fmt.Fprintln(f, line)
	}
	return f.Name()
}

func TestLoadWordlist_Basic(t *testing.T) {
	path := writeWordlist(t, []string{"admin", "root", "test"})
	words, err := LoadWordlist(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"admin", "root", "test"}; !equalSlices(words, want) {
		t.Errorf("got %v, want %v", words, want)
	}
}

func TestLoadWordlist_SkipsBlankLines(t *testing.T) {
	path := writeWordlist(t, []string{"admin", "", "  ", "root", ""})
	words, err := LoadWordlist(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"admin", "root"}; !equalSlices(words, want) {
		t.Errorf("got %v, want %v", words, want)
	}
}

func TestLoadWordlist_MissingFile(t *testing.T) {
	_, err := LoadWordlist(t.TempDir() + "/nonexistent.txt")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestAssignFuzzKeys_OneWordlist(t *testing.T) {
	keys := AssignFuzzKeys(1)
	if want := []string{"FUZZ"}; !equalSlices(keys, want) {
		t.Errorf("got %v, want %v", keys, want)
	}
}

func TestAssignFuzzKeys_TwoWordlists(t *testing.T) {
	keys := AssignFuzzKeys(2)
	if want := []string{"FUZZ1", "FUZZ2"}; !equalSlices(keys, want) {
		t.Errorf("got %v, want %v", keys, want)
	}
}

func TestAssignFuzzKeys_ThreeWordlists(t *testing.T) {
	keys := AssignFuzzKeys(3)
	if want := []string{"FUZZ1", "FUZZ2", "FUZZ3"}; !equalSlices(keys, want) {
		t.Errorf("got %v, want %v", keys, want)
	}
}

func TestSubstitutePayload_SingleKey(t *testing.T) {
	got := SubstitutePayload("http://example.com/FUZZ", []string{"FUZZ"}, []string{"admin"})
	if want := "http://example.com/admin"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstitutePayload_MultipleKeys(t *testing.T) {
	got := SubstitutePayload(
		"http://example.com/FUZZ1/FUZZ2",
		[]string{"FUZZ1", "FUZZ2"},
		[]string{"api", "v1"},
	)
	if want := "http://example.com/api/v1"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstitutePayload_InBody(t *testing.T) {
	got := SubstitutePayload(`{"username":"FUZZ"}`, []string{"FUZZ"}, []string{"admin"})
	if want := `{"username":"admin"}`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// equalSlices reports whether two string slices are equal element-by-element.
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
