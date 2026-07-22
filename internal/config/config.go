package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds every setting the fr3ki tool supports. Zero values mean "not
// set"; the CLI layer supplies hardcoded defaults on top of this.
type Config struct {
	Wordlists      []string `yaml:"wordlists"`
	Threads        int      `yaml:"threads"`
	Output         string   `yaml:"output"`
	Format         string   `yaml:"format"`
	Rate           int      `yaml:"rate"`
	Cooldown       int      `yaml:"cooldown"`
	Debug          bool     `yaml:"debug"`
	Obfuscate      bool     `yaml:"obfuscate"`
	Verbose        bool     `yaml:"verbose"`
	Proxies        string   `yaml:"proxies"`
	Headers        []string `yaml:"headers"`
	Method         string   `yaml:"method"`
	FilterCodes    []int    `yaml:"filter_codes"`
	FilterSizes    []int    `yaml:"filter_sizes"`
	MatchSizes     []int    `yaml:"match_sizes"`
	FilterWords    []int    `yaml:"filter_words"`
	MatchWords     []int    `yaml:"match_words"`
	MatchString    string   `yaml:"match_string"`
	MatchRegex     string   `yaml:"match_regex"`
	Data           string   `yaml:"data"`
	Resume         bool     `yaml:"resume"`
	Recursive      bool     `yaml:"recursive"`
	RecursionDepth int      `yaml:"recursion_depth"`
}

// LoadConfig reads a YAML config file into a Config. If the file does not
// exist it prints a warning and returns a zero-value Config, matching the
// Python tool's "warn and continue" behaviour.
func LoadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Config file not found. Using default settings.\n")
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config %q: %w", path, err)
	}
	return cfg, nil
}

// Merge returns a new Config where every non-zero field in flags overrides the
// corresponding field in c. All merge logic lives here rather than at call
// sites.
func (c Config) Merge(flags Config) Config {
	result := c

	if len(flags.Wordlists) > 0 {
		result.Wordlists = flags.Wordlists
	}
	if flags.Threads != 0 {
		result.Threads = flags.Threads
	}
	if flags.Output != "" {
		result.Output = flags.Output
	}
	if flags.Format != "" {
		result.Format = flags.Format
	}
	if flags.Rate != 0 {
		result.Rate = flags.Rate
	}
	if flags.Cooldown != 0 {
		result.Cooldown = flags.Cooldown
	}
	if flags.Debug {
		result.Debug = true
	}
	if flags.Obfuscate {
		result.Obfuscate = true
	}
	if flags.Verbose {
		result.Verbose = true
	}
	if flags.Proxies != "" {
		result.Proxies = flags.Proxies
	}
	if len(flags.Headers) > 0 {
		result.Headers = flags.Headers
	}
	if flags.Method != "" {
		result.Method = flags.Method
	}
	if len(flags.FilterCodes) > 0 {
		result.FilterCodes = flags.FilterCodes
	}
	if len(flags.FilterSizes) > 0 {
		result.FilterSizes = flags.FilterSizes
	}
	if len(flags.MatchSizes) > 0 {
		result.MatchSizes = flags.MatchSizes
	}
	if len(flags.FilterWords) > 0 {
		result.FilterWords = flags.FilterWords
	}
	if len(flags.MatchWords) > 0 {
		result.MatchWords = flags.MatchWords
	}
	if flags.MatchString != "" {
		result.MatchString = flags.MatchString
	}
	if flags.MatchRegex != "" {
		result.MatchRegex = flags.MatchRegex
	}
	if flags.Data != "" {
		result.Data = flags.Data
	}
	if flags.Resume {
		result.Resume = true
	}
	if flags.Recursive {
		result.Recursive = true
	}
	if flags.RecursionDepth != 0 {
		result.RecursionDepth = flags.RecursionDepth
	}

	return result
}
