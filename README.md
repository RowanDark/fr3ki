# fr3ki

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/T6T61FKEIB)

**fr3ki** is an advanced asynchronous fuzzer designed for bug bounty hunters, penetration testers, and red teamers. It features high concurrency, payload obfuscation, proxy rotation, adaptive throttling, and much more—all in a single extensible Python tool.

**NOTE** Only use this on programs and applications that you are authorized to perform research and testing on!  Failure to do so is considered illegal in most jurisdictions, and you do so at your own risk!

---

## Features

- **High-speed asynchronous fuzzing** with adjustable concurrency and rate limits
- **Context-aware engine** adapts to response codes, throttles, and backs off on 429/403 to evade WAFs
- **Payload obfuscation**: Toggleable multi-style (URL, base64, hex, unicode, double-encode, etc.)
- **Proxy & header rotation** for stealth (supports proxies.txt, random User-Agents, custom headers via `-A`)
- **Incremental result saving**: No data loss on interruption; each response logged live
- **Live color CLI output** with `rich`—see status codes and progress at a glance
- **YAML config support** and CLI overrides for all options
- **Auto venv check** and user-friendly install guidance
- **HTTP method selection** via `--method` (GET, POST, PUT, DELETE, HEAD, OPTIONS, PATCH)
- **Status code filtering** via `--filter-code` to suppress uninteresting responses (e.g. 404s)
- **Response size filtering** via `--filter-size` / `--match-size` to eliminate false positives by byte length
- **Word count filtering** via `--filter-words` / `--match-words` to filter by response word count
- **Response body matching** via `--match-string` / `--match-regex` to find specific content in responses
- **POST body fuzzing** via `--data` with FUZZ keyword support in request bodies
- **Multiple FUZZ positions** with separate wordlists (FUZZ1, FUZZ2, etc.) using `itertools.product`
- **Resume support** via `--resume` to continue interrupted fuzzing runs
- **Multiple output formats** via `--format` (JSONL, CSV, HTML)
- **Recursive directory fuzzing** via `--recursive` to automatically fuzz discovered paths
- **Extensible**: Built by bug bounty hunters, for bug bounty hunters!

---

## Installation

```bash
git clone https://github.com/RowanDark/fr3ki.git
cd fr3ki
python3 -m venv fr3ki_env
source fr3ki_env/bin/activate
pip install -r requirements.txt
```

## Wordlists

fr3ki requires a wordlist to operate. Recommended sources:
- [SecLists](https://github.com/danielmiessler/SecLists) — the gold standard for fuzzing wordlists
- [fuzz.txt](https://github.com/Bo0oM/fuzz.txt) — focused on common web paths
- [assetnote wordlists](https://wordlists.assetnote.io/) — regularly updated, tech-specific lists

Place your wordlist in a `wordlists/` subdirectory or pass the full path with `-w`.

## Usage

Basic Example:

```bash
python3 fr3ki.py -u https://target.com/FUZZ -w wordlists/common.txt -o results.json
```

## Useful Flags:

```
-w wordlist.txt       : Wordlist file (can repeat for multi-position fuzzing)
-t 10                 : Limit concurrency (threads)
-o results.json       : Output results file
--rate 5              : Global rate limit of 5 requests/sec across all threads (0 = unlimited)
--debug               : Save all responses, not just interesting
--verbose             : Include 200-char response snippets
--obfuscate           : Use obfuscated payloads
--proxies proxies.txt : Use rotating proxies from a file
-A "Header:Value"     : Add custom header(s) (can repeat)
--cooldown 15         : Wait 15s after 429 error
--method POST         : Use a specific HTTP method (default: GET)
--filter-code 403     : Suppress 403 responses from output (default: 404; can repeat)
--filter-size 1234    : Hide responses of exactly 1234 bytes (can repeat)
--match-size 4096     : Only show responses of exactly 4096 bytes (can repeat)
--filter-words 5      : Hide responses with exactly 5 words (can repeat)
--match-words 100     : Only show responses with exactly 100 words (can repeat)
--match-string "admin": Only show responses containing "admin"
--match-regex "v\d+\.\d+" : Only show responses matching a regex pattern
--data "param=FUZZ"   : POST body data with FUZZ keyword for body fuzzing
--resume              : Resume from existing output file, skip already-tested URLs
--format csv          : Output format: jsonl (default), csv, or html
--recursive           : Automatically fuzz discovered directories
--recursion-depth 3   : Maximum recursion depth (default: 2)
```

## Example with Multiple Flags:

```bash
python3 fr3ki.py -u https://target.com/FUZZ -w wordlists/common.txt -o results.json --rate 1 --debug --obfuscate --proxies proxies.txt -A "Authorization: Bearer xyz"
```

## Advanced Usage Examples

### Multi-Position Fuzzing

Fuzz two positions simultaneously with separate wordlists. Use `FUZZ1` and `FUZZ2` in the URL and provide two `-w` flags:

```bash
python3 fr3ki.py -u https://target.com/FUZZ1/FUZZ2 -w wordlists/dirs.txt -w wordlists/files.txt -o results.json
```

This tests every combination of directories and files (cartesian product).

### POST Body Fuzzing

Fuzz login forms, API endpoints, or any POST parameter. The FUZZ keyword works in the `--data` body:

```bash
python3 fr3ki.py -u https://target.com/login -w wordlists/passwords.txt --method POST --data "username=admin&password=FUZZ" -o login_results.json
```

You can also combine URL and body fuzzing with multi-position mode:

```bash
python3 fr3ki.py -u https://target.com/FUZZ1 -w wordlists/endpoints.txt -w wordlists/params.txt --method POST --data "key=FUZZ2" -o api_results.json
```

### Recursive Directory Fuzzing

Automatically fuzz into discovered directories. When fr3ki finds a 200 or redirect on a directory-like path, it queues that path for another fuzzing pass:

```bash
python3 fr3ki.py -u https://target.com/FUZZ -w wordlists/common.txt --recursive --recursion-depth 3 -o recursive_results.json
```

### HTML Report Generation

Generate a styled HTML report instead of JSONL:

```bash
python3 fr3ki.py -u https://target.com/FUZZ -w wordlists/common.txt --format html -o report.html
```

CSV output is also available:

```bash
python3 fr3ki.py -u https://target.com/FUZZ -w wordlists/common.txt --format csv -o results.csv
```

### Resume an Interrupted Scan

If a scan is interrupted, resume from where you left off. Already-tested URLs are loaded from the output file and skipped:

```bash
python3 fr3ki.py -u https://target.com/FUZZ -w wordlists/common.txt -o results.json --resume
```

### Response Filtering Examples

Filter out false positives by response size (e.g., a custom 404 page that's always 1234 bytes):

```bash
python3 fr3ki.py -u https://target.com/FUZZ -w wordlists/common.txt --filter-size 1234
```

Find responses containing a specific string or regex pattern:

```bash
python3 fr3ki.py -u https://target.com/FUZZ -w wordlists/common.txt --match-string "admin panel"
python3 fr3ki.py -u https://target.com/FUZZ -w wordlists/common.txt --match-regex "version\s+\d+\.\d+"
```

## Proxy file format (proxies.txt):

```
http://127.0.0.1:8080
https://proxy.example.com:3128
```

## Example Output

```json
{"url": "https://target.com/admin", "status_code": 200, "length": 4096, "words": 312}
{"url": "https://target.com/login", "status_code": 403, "length": 78, "words": 8}
```

If `--verbose` is enabled, a `"snippet"` field is included for each entry.

## Configuration File (fr3ki_config.yaml)

You can store default options in `fr3ki_config.yaml`:

```yaml
wordlist: wordlists/common.txt
threads: 10
output: fr3ki_results.json
proxies: proxies.txt
headers:
  - "X-Api-Key: testkey"
filter_sizes: []
match_sizes: []
filter_words: []
match_words: []
```

Any CLI flag will override config file values.


## Support My Work

If you found fr3ki useful, please consider supporting via Ko-fi!

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/T6T61FKEIB)

## License

MIT License — see LICENSE file.

Developed with 🐺 by RowanDark, 2025.
