package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/RowanDark/fr3ki/internal/config"
)

const version = "0.1.0-go"

// stringSliceFlag accumulates repeated string flags (-w a -w b → ["a","b"]).
type stringSliceFlag []string

func (f *stringSliceFlag) String() string { return strings.Join(*f, ", ") }
func (f *stringSliceFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// intSliceFlag accumulates repeated int flags.
type intSliceFlag []int

func (f *intSliceFlag) String() string {
	parts := make([]string, len(*f))
	for i, n := range *f {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}

func (f *intSliceFlag) Set(v string) error {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("%q: not a valid integer", v)
	}
	*f = append(*f, n)
	return nil
}

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true, "PATCH": true,
}

var validFormats = map[string]bool{
	"jsonl": true, "csv": true, "html": true,
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("fr3ki", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: fr3ki -u <url> [flags]\n\n")
		fs.PrintDefaults()
	}

	versionFlag := fs.Bool("version", false, "print version and exit")

	// Target URL — required, not stored in Config.
	var urlFlag string
	fs.StringVar(&urlFlag, "u", "", "Target URL with FUZZ keyword (required)")
	fs.StringVar(&urlFlag, "url", "", "Target URL with FUZZ keyword (required)")

	// Wordlists
	var wordlists stringSliceFlag
	fs.Var(&wordlists, "w", "Wordlist file (repeatable; use multiple -w for FUZZ1, FUZZ2, …)")
	fs.Var(&wordlists, "wordlist", "Wordlist file (repeatable)")

	// Scalar flags with short+long aliases sharing the same variable.
	var threads int
	fs.IntVar(&threads, "t", 0, "Max concurrent requests (default: 10)")
	fs.IntVar(&threads, "threads", 0, "Max concurrent requests (default: 10)")

	var output string
	fs.StringVar(&output, "o", "", "Output file path (default: fr3ki_results.json)")
	fs.StringVar(&output, "output", "", "Output file path (default: fr3ki_results.json)")

	// Timing and behaviour
	rate := fs.Int("rate", 0, "Max requests per second (0 = unlimited)")
	cooldown := fs.Int("cooldown", 0, "Cooldown seconds after a 429 response (default: 10)")
	debug := fs.Bool("debug", false, "Save all responses, not just interesting ones")
	obfuscate := fs.Bool("obfuscate", false, "Enable payload obfuscation")
	verbose := fs.Bool("verbose", false, "Include a response snippet in output")

	// Network / protocol
	proxies := fs.String("proxies", "", "File containing a list of proxies, one per line")

	var headers stringSliceFlag
	fs.Var(&headers, "A", `Custom header, e.g. -A "X-Token:abc" (repeatable)`)
	fs.Var(&headers, "header", "Custom header (repeatable)")

	method := fs.String("method", "", "HTTP method: GET POST PUT DELETE HEAD OPTIONS PATCH (default: GET)")

	// Filter / match flags
	var filterCodes intSliceFlag
	fs.Var(&filterCodes, "filter-code", "Status code to suppress from output (repeatable; default: 404)")

	var filterSizes intSliceFlag
	fs.Var(&filterSizes, "filter-size", "Hide responses of exactly this byte length (repeatable)")

	var matchSizes intSliceFlag
	fs.Var(&matchSizes, "match-size", "Only show responses of exactly this byte length (repeatable)")

	var filterWords intSliceFlag
	fs.Var(&filterWords, "filter-words", "Hide responses with exactly this word count (repeatable)")

	var matchWords intSliceFlag
	fs.Var(&matchWords, "match-words", "Only show responses with exactly this word count (repeatable)")

	matchString := fs.String("match-string", "", "Only show responses containing this string")
	matchRegex := fs.String("match-regex", "", "Only show responses matching this regex")

	// POST body
	data := fs.String("data", "", `POST body data with FUZZ keyword (e.g. "username=FUZZ&password=test")`)

	// Control flow
	resume := fs.Bool("resume", false, "Resume from an existing output file (jsonl only)")
	format := fs.String("format", "", "Output format: jsonl (default), csv, or html")
	recursive := fs.Bool("recursive", false, "Automatically fuzz discovered directories")
	recursionDepth := fs.Int("recursion-depth", 0, "Max recursion depth (default: 2)")

	if err := fs.Parse(args); err != nil {
		// ContinueOnError: fs already printed the error.
		return 1
	}

	if *versionFlag {
		fmt.Fprintln(stdout, "fr3ki", version)
		return 0
	}

	// Load YAML config — a missing file is a warning, not a fatal error.
	cfg, err := config.LoadConfig("fr3ki_config.yaml")
	if err != nil {
		fmt.Fprintf(stderr, "fr3ki: %v\n", err)
		return 1
	}

	// Build a Config from parsed flags; zero values mean "not set by CLI".
	flags := config.Config{
		Wordlists:      []string(wordlists),
		Threads:        threads,
		Output:         output,
		Format:         *format,
		Rate:           *rate,
		Cooldown:       *cooldown,
		Debug:          *debug,
		Obfuscate:      *obfuscate,
		Verbose:        *verbose,
		Proxies:        *proxies,
		Headers:        []string(headers),
		Method:         *method,
		FilterCodes:    []int(filterCodes),
		FilterSizes:    []int(filterSizes),
		MatchSizes:     []int(matchSizes),
		FilterWords:    []int(filterWords),
		MatchWords:     []int(matchWords),
		MatchString:    *matchString,
		MatchRegex:     *matchRegex,
		Data:           *data,
		Resume:         *resume,
		Recursive:      *recursive,
		RecursionDepth: *recursionDepth,
	}

	merged := cfg.Merge(flags)

	// Apply hardcoded defaults for fields still at zero after merging.
	if merged.Threads == 0 {
		merged.Threads = 10
	}
	if merged.Output == "" {
		merged.Output = "fr3ki_results.json"
	}
	if merged.Format == "" {
		merged.Format = "jsonl"
	}
	if merged.Method == "" {
		merged.Method = "GET"
	}
	if merged.Cooldown == 0 {
		merged.Cooldown = 10
	}
	if merged.RecursionDepth == 0 {
		merged.RecursionDepth = 2
	}
	if len(merged.FilterCodes) == 0 {
		merged.FilterCodes = []int{404}
	}
	if len(merged.Wordlists) == 0 {
		merged.Wordlists = []string{"wordlists/common.txt"}
	}

	// --- Validation ---

	if urlFlag == "" {
		fmt.Fprintln(stderr, "fr3ki: -u/--url is required")
		return 1
	}

	if !strings.Contains(urlFlag+merged.Data, "FUZZ") {
		fmt.Fprintln(stderr, "fr3ki: FUZZ keyword must appear in the URL or --data body")
		return 1
	}

	if !validMethods[merged.Method] {
		fmt.Fprintf(stderr, "fr3ki: invalid --method %q; choose from GET POST PUT DELETE HEAD OPTIONS PATCH\n", merged.Method)
		return 1
	}

	if !validFormats[merged.Format] {
		fmt.Fprintf(stderr, "fr3ki: invalid --format %q; choose from jsonl, csv, html\n", merged.Format)
		return 1
	}

	// Warnings — continue after printing.
	if merged.Resume && merged.Format != "jsonl" {
		fmt.Fprintln(stderr, "fr3ki: warning: --resume only supports --format jsonl; resume data will not be loaded")
	}
	if merged.Data != "" && merged.Method == "GET" {
		fmt.Fprintln(stderr, "fr3ki: warning: --data provided with GET method; consider using --method POST")
	}

	// Fuzzing stub: print merged config (plus the URL) and exit 0.
	out := struct {
		URL string `json:"url"`
		config.Config
	}{URL: urlFlag, Config: merged}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(stderr, "fr3ki: %v\n", err)
		return 1
	}
	return 0
}
