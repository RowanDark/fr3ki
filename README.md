# fr3ki

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/T6T61FKEIB)

**fr3ki** is an advanced asynchronous fuzzer designed for bug bounty hunters, penetration testers, and red teamers. It features high concurrency, payload obfuscation, proxy rotation, adaptive throttling, and much more—all in a single extensible Python tool.

**NOTE** Only use this on programs and applications that you are authorized to perform research and testing on!  Failure to do so is considered illegal in most jurisdictions, and you do so at your own risk!

---

## Features

- 🚀 **High-speed asynchronous fuzzing** with adjustable concurrency and rate limits
- 🧠 **Context-aware engine** adapts to response codes, throttles, and backs off on 429/403 to evade WAFs
- 🕵️ **Payload obfuscation**: Toggleable multi-style (URL, base64, hex, unicode, double-encode, etc.)
- 🎭 **Proxy & header rotation** for stealth (supports proxies.txt, random User-Agents, custom headers via `-A`)
- 💾 **Incremental result saving**: No data loss on interruption; each response logged live
- 🎨 **Live color CLI output** with `rich`—see status codes and progress at a glance
- 📂 **YAML config support** and CLI overrides for all options
- 🐍 **Auto venv check** and user-friendly install guidance
- 🔀 **HTTP method selection** via `--method` (GET, POST, PUT, DELETE, HEAD, OPTIONS, PATCH)
- 🔇 **Status code filtering** via `--filter-code` to suppress uninteresting responses (e.g. 404s)
- 🛠️ **Extensible**: Built by bug bounty hunters, for bug bounty hunters!

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
--rate 5          : Global rate limit of 5 requests/sec across all threads (0 = unlimited)
-t 10             : Limit concurrency (threads)
--debug           : Save all responses, not just interesting
--verbose         : Include 200-char response snippets
--obfuscate       : Use obfuscated payloads
--proxies proxies.txt : Use rotating proxies from a file
-A "Header:Value" : Add custom header(s) (can repeat)
--cooldown 15     : Wait 15s after 429 error
--method POST     : Use a specific HTTP method (default: GET)
--filter-code 403 : Suppress 403 responses from output (default: 404; can repeat)
```

## Example with Multiple Flags:

```bash
python3 fr3ki.py -u https://target.com/FUZZ -w wordlists/common.txt -o results.json --rate 1 --debug --obfuscate --proxies proxies.txt -A "Authorization: Bearer xyz"
```

## Proxy file format (proxies.txt):

```
http://127.0.0.1:8080
https://proxy.example.com:3128
```

## Example Output

```json
{"url": "https://target.com/admin", "status_code": 200, "length": 4096}
{"url": "https://target.com/login", "status_code": 403, "length": 78}
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
```

Any CLI flag will override config file values.


## Support My Work

If you found fr3ki useful, please consider supporting via Ko-fi!

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/T6T61FKEIB)

## License

MIT License — see LICENSE file.

Developed with 🐺 by RowanDark, 2025.
