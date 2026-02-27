#!/usr/bin/env python3

import sys
import os
import argparse
import asyncio
import random
import time
import json
import base64
import urllib.parse

# Fix #2: venv check BEFORE rich import, using only builtin print
def _check_venv():
    if sys.prefix == sys.base_prefix:
        print("\nWarning: You are NOT in a virtual environment.")
        print("For best results, run:")
        print("    python3 -m venv fr3ki_env")
        print("    source fr3ki_env/bin/activate")
        print("    pip install -r requirements.txt\n")
        print("Or install dependencies with:")
        print("    pip install rich httpx pyyaml\n")
        print("Then re-run this script!")
        sys.exit(1)

_check_venv()

try:
    from rich import print
    from rich.progress import Progress
except ImportError:
    print("You need the 'rich' library for color output.\nInstall with: pip install rich\n")
    sys.exit(1)

import httpx
import yaml
import re
import itertools

# Banner
def print_banner():
    print(r"""
    ______    _____ __   _
   / ____/___|__  // /__(_)
  / /_  / ___//_ </ //_/ /
 / __/ / /  ___/ / ,< / /
/_/   /_/  /____/_/|_/_/

          fr3ki   © 2025 [bold red]RowanDark[/bold red]

                 .
                / V\
              / `  /
             <<   |
             /    |
           /      |
         /        |
       /    \  \ /
      (      ) | |
  ____|   _/_  | |
<______\______)\__)
""")

def load_config(config_file):
    try:
        with open(config_file, 'r') as file:
            config = yaml.safe_load(file)
            return config if config else {}
    except FileNotFoundError:
        print("[yellow]Config file not found. Using default settings.[/yellow]")
        return {}

def chunked(lst, n):
    for i in range(0, len(lst), n):
        yield lst[i:i + n]

def obfuscate_payload(word):
    payloads = [word]
    payloads.append(urllib.parse.quote(word))
    payloads.append(base64.b64encode(word.encode()).decode())
    payloads.append(''.join(['%{:02x}'.format(ord(c)) for c in word]))
    payloads.append(word.replace('/', '%2F').replace(':', '%3A'))
    payloads.append(''.join(['\\u{:04x}'.format(ord(c)) for c in word]))
    payloads.append(urllib.parse.quote(urllib.parse.quote(word)))
    return list(set(payloads))

def count_words(text):
    return len(text.split())

def incremental_save(entry, filename, fmt='jsonl'):
    if fmt == 'jsonl':
        with open(filename, 'a') as f:
            f.write(json.dumps(entry) + "\n")
    elif fmt == 'csv':
        import csv
        file_exists = os.path.exists(filename)
        with open(filename, 'a', newline='') as f:
            writer = csv.DictWriter(f, fieldnames=entry.keys())
            if not file_exists:
                writer.writeheader()
            writer.writerow(entry)
    elif fmt == 'html':
        with open(filename, 'a') as f:
            row = "<tr>" + "".join(f"<td>{v}</td>" for v in entry.values()) + "</tr>\n"
            f.write(row)

def init_html_output(filename, columns):
    with open(filename, 'w') as f:
        f.write("""<!DOCTYPE html>
<html><head><title>fr3ki Results</title>
<style>
body { font-family: monospace; background: #1a1a1a; color: #e0e0e0; }
table { width: 100%; border-collapse: collapse; }
th { background: #333; padding: 8px; text-align: left; }
td { padding: 6px 8px; border-bottom: 1px solid #333; }
tr:hover { background: #2a2a2a; }
.s200 { color: #4caf50; } .s301 { color: #03a9f4; }
.s401 { color: #ff9800; } .s403 { color: #ff9800; }
.s500 { color: #f44336; font-weight: bold; }
</style></head><body>
<h2>fr3ki Fuzzing Results</h2>
<table><thead><tr>""")
        for col in columns:
            f.write(f"<th>{col}</th>")
        f.write("</tr></thead><tbody>\n")

def finalize_html_output(filename):
    with open(filename, 'a') as f:
        f.write("</tbody></table></body></html>\n")

def load_proxies(proxy_file):
    try:
        with open(proxy_file, 'r') as f:
            return [line.strip() for line in f if line.strip()]
    except FileNotFoundError:
        print("[yellow]Proxy file not found. Proceeding without proxies.[/yellow]")
        return []

def get_random_proxy(proxies):
    if proxies:
        return random.choice(proxies)
    return None

def random_user_agent():
    agents = [
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/91.0.4472.124 Safari/537.36",
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/537.36",
        "Mozilla/5.0 (X11; Ubuntu; Linux x86_64) Gecko/20100101 Firefox/89.0",
        "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148",
        "Mozilla/5.0 (Linux; Android 11; Pixel 4) AppleWebKit/537.36 Chrome/91.0.4472.124 Mobile Safari/537.36"
    ]
    return random.choice(agents)

def generate_headers(custom_headers=None):
    headers = {
        "User-Agent": random_user_agent(),
        "Accept": "*/*",
        "Accept-Language": "en-US,en;q=0.9",
        "Connection": "keep-alive"
    }
    if custom_headers:
        for h in custom_headers:
            parts = h.split(":", 1)
            if len(parts) == 2:
                k, v = parts
                headers[k.strip()] = v.strip()
    return headers


# Fix #4: Global rate limiter class
class RateLimiter:
    def __init__(self, rate):
        self.rate = rate  # requests per second
        self.tokens = rate
        self.last_refill = time.monotonic()
        self.lock = asyncio.Lock()

    async def acquire(self):
        if self.rate <= 0:
            return
        async with self.lock:
            now = time.monotonic()
            elapsed = now - self.last_refill
            self.tokens = min(self.rate, self.tokens + elapsed * self.rate)
            self.last_refill = now
            if self.tokens < 1:
                wait = (1 - self.tokens) / self.rate
                await asyncio.sleep(wait)
                self.tokens = 0
            else:
                self.tokens -= 1


# Fix #9: Interesting status codes set
INTERESTING_CODES = {200, 201, 202, 204, 301, 302, 307, 308, 401, 403, 405, 500}


async def fr3ki_fuzzer(
    base_url, wordlists, threads, verbose, output, obfuscate, rate,
    cooldown, debug, proxy_file, custom_headers, method, filter_codes,
    filter_sizes, match_sizes, filter_words, match_words,
    match_string, match_regex, data, resume, fmt, recursive, recursion_depth
):
    proxies = load_proxies(proxy_file) if proxy_file else []

    # Load all wordlists
    all_words = []
    for wl_path in wordlists:
        with open(wl_path) as f:
            all_words.append([line.strip() for line in f if line.strip()])

    # Determine FUZZ keywords to replace
    fuzz_keys = []
    for i in range(len(wordlists)):
        key = f"FUZZ{i+1}" if len(wordlists) > 1 else "FUZZ"
        fuzz_keys.append(key)

    # Handle obfuscation by expanding wordlists
    if obfuscate:
        expanded = []
        for words in all_words:
            expanded_words = []
            for w in words:
                expanded_words.extend(obfuscate_payload(w))
            expanded.append(expanded_words)
        all_words = expanded

    # Generate payload combinations
    if len(wordlists) == 1:
        payload_sets = [(w,) for w in all_words[0]]
    else:
        payload_sets = list(itertools.product(*all_words))

    sem = asyncio.Semaphore(threads)
    # Fix #4: instantiate global rate limiter
    limiter = RateLimiter(rate)
    # Fix #12: counters for summary
    counter = [0, 0]  # [total_requests, interesting_results]

    # Resume support
    tested_urls = set()
    if resume and os.path.exists(output):
        try:
            with open(output, 'r') as f:
                for line in f:
                    try:
                        entry = json.loads(line.strip())
                        if 'url' in entry:
                            tested_urls.add(entry['url'])
                    except json.JSONDecodeError:
                        continue
            print(f"[cyan]Resuming: loaded {len(tested_urls)} already-tested URLs from {output}[/cyan]")
        except Exception as e:
            print(f"[yellow]Could not load resume file: {e}[/yellow]")

    # Recursive mode setup
    discovery_queue = asyncio.Queue()
    discovered_paths = set()

    # HTML output initialization
    if fmt == 'html':
        if not (resume and os.path.exists(output)):
            columns = ["url", "status_code", "length", "words"]
            if verbose:
                columns.append("snippet")
            init_html_output(output, columns)

    # Shared client for requests without a proxy (preserves connection pooling)
    shared_client = httpx.AsyncClient(timeout=10, follow_redirects=True)

    async def fetch_url(base_url_local, payload_set, proxy, headers, current_depth=0, fuzz_keys_local=None):
        if fuzz_keys_local is None:
            fuzz_keys_local = fuzz_keys

        url = base_url_local
        body = data
        for key, payload in zip(fuzz_keys_local, payload_set):
            url = url.replace(key, payload)
            if body:
                body = body.replace(key, payload)

        # Resume check
        if url in tested_urls:
            return

        async with sem:
            # Fix #4: acquire global rate limiter
            await limiter.acquire()
            # Fix #10: per-request jitter for evasion
            await asyncio.sleep(random.uniform(0.05, 0.3))
            try:
                if proxy:
                    proxy_map = {"http://": proxy, "https://": proxy}
                    async with httpx.AsyncClient(timeout=10, follow_redirects=True, proxies=proxy_map) as client:
                        resp = await client.request(method, url, headers=headers, content=body)
                else:
                    resp = await shared_client.request(method, url, headers=headers, content=body)
                counter[0] += 1
                response_size = len(resp.content)

                # Size filtering (Issue 1)
                if filter_sizes and response_size in filter_sizes:
                    return
                if match_sizes and response_size not in match_sizes:
                    return

                # Word count filtering (Issue 2)
                word_count = count_words(resp.text)
                if filter_words and word_count in filter_words:
                    return
                if match_words and word_count not in match_words:
                    return

                # String/regex matching (Issue 3)
                if match_string and match_string not in resp.text:
                    return
                if match_regex:
                    try:
                        if not re.search(match_regex, resp.text):
                            return
                    except re.error as e:
                        print(f"[red]Invalid regex pattern: {e}[/red]")
                        return

                entry = {
                    "url": url,
                    "status_code": resp.status_code,
                    "length": response_size,
                    "words": word_count
                }
                if verbose:
                    entry["snippet"] = resp.text[:200]
                # Fix #9: improved interesting filter
                if debug or resp.status_code in INTERESTING_CODES or response_size > 500:
                    incremental_save(entry, output, fmt)
                    counter[1] += 1
                if resp.status_code == 429:
                    retry_after = resp.headers.get('Retry-After')
                    cooldown_time = int(retry_after) if retry_after and retry_after.isdigit() else cooldown
                    print(f"[yellow]429 received, cooling down for {cooldown_time} seconds.[/yellow]")
                    await asyncio.sleep(cooldown_time)
                elif resp.status_code == 403:
                    print(f"[yellow]403 received for {url}, backing off for {cooldown // 2} seconds.[/yellow]")
                    await asyncio.sleep(cooldown // 2)

                if resp.status_code not in filter_codes:
                    # Match highlight (Issue 3)
                    if match_string or match_regex:
                        print(f"[bold magenta]✓ MATCH: {url} [{resp.status_code}] ({response_size} bytes)[/bold magenta]")

                    if resp.status_code in {200, 201, 202, 204}:
                        print(f"[green]{url} [{resp.status_code}][/green]")
                    elif resp.status_code in {301, 302, 307, 308}:
                        print(f"[cyan]{url} [{resp.status_code} Redirect][/cyan]")
                    elif resp.status_code == 401:
                        print(f"[yellow]{url} [{resp.status_code} Unauthorized][/yellow]")
                    elif resp.status_code == 403:
                        print(f"[yellow]{url} [{resp.status_code} Forbidden][/yellow]")
                    elif resp.status_code == 404:
                        print(f"[dim]{url} [{resp.status_code} Not Found][/dim]")
                    elif resp.status_code == 405:
                        print(f"[yellow]{url} [{resp.status_code} Method Not Allowed][/yellow]")
                    elif resp.status_code in {500, 502, 503}:
                        print(f"[bold red]{url} [{resp.status_code} Server Error][/bold red]")
                    else:
                        print(f"[red]{url} [{resp.status_code}][/red]")

                # Recursive discovery (Issue 8)
                if recursive and resp.status_code in {200, 301, 302}:
                    path = urllib.parse.urlparse(url).path
                    if path.endswith('/') or '.' not in path.split('/')[-1]:
                        new_base = url.rstrip('/') + '/FUZZ'
                        if new_base not in discovered_paths:
                            discovered_paths.add(new_base)
                            await discovery_queue.put((new_base, current_depth + 1))

            except httpx.TimeoutException:
                print(f"[yellow]Timeout: {url}[/yellow]")
            except httpx.ConnectError:
                print(f"[red]Connection error: {url}[/red]")
            except Exception as e:
                print(f"[red]Error with {url}: {e}[/red]")

    # Main fuzzing loop
    with Progress() as progress:
        task = progress.add_task("[cyan]Fuzzing with fr3ki...[/cyan]", total=len(payload_sets))
        for chunk in chunked(payload_sets, threads):
            tasks = []
            for payload_set in chunk:
                proxy = get_random_proxy(proxies)
                headers = generate_headers(custom_headers)
                tasks.append(fetch_url(base_url, payload_set, proxy, headers))
            await asyncio.gather(*tasks)
            progress.update(task, advance=len(chunk))
            # Fix #10: removed chunk-level sleep (jitter is per-request now)

    # Recursive fuzzing (Issue 8)
    while not discovery_queue.empty():
        new_base_url, depth = await discovery_queue.get()
        if depth > recursion_depth:
            continue
        print(f"\n[bold cyan]→ Recursing into: {new_base_url}[/bold cyan]")
        recursive_payloads = [(w,) for w in all_words[0]]
        with Progress() as progress:
            task = progress.add_task(f"[cyan]Recursing: {new_base_url}[/cyan]", total=len(recursive_payloads))
            for chunk in chunked(recursive_payloads, threads):
                tasks = []
                for payload_set in chunk:
                    proxy = get_random_proxy(proxies)
                    headers = generate_headers(custom_headers)
                    tasks.append(fetch_url(new_base_url, payload_set, proxy, headers, depth, ["FUZZ"]))
                await asyncio.gather(*tasks)
                progress.update(task, advance=len(chunk))

    await shared_client.aclose()

    # Finalize HTML output
    if fmt == 'html':
        finalize_html_output(output)

    # Fix #12: output summary
    print(f"\n[bold green]✓ Fuzzing complete.[/bold green] {counter[0]} requests sent. Results saved to [cyan]{output}[/cyan]")


def main():
    print_banner()
    config = load_config('fr3ki_config.yaml')
    parser = argparse.ArgumentParser(description="fr3ki - Advanced Fuzzer by RowanDark")
    parser.add_argument('-u', '--url', required=True, help='Target URL with FUZZ keyword')
    parser.add_argument('-w', '--wordlist', action='append', dest='wordlists', default=None,
                        help='Wordlist file. Use multiple -w flags for multiple FUZZ positions (FUZZ1, FUZZ2, etc.)')
    parser.add_argument('-t', '--threads', type=int, default=config.get('threads', 10), help='Max concurrent requests')
    parser.add_argument('-o', '--output', default=config.get('output', 'fr3ki_results.json'), help='Output results file')
    parser.add_argument('--rate', type=int, default=0, help='Requests per second (0=unlimited)')
    parser.add_argument('--cooldown', type=int, default=10, help='Cooldown (seconds) after 429')
    parser.add_argument('--debug', action='store_true', help='Save all responses, not just interesting')
    parser.add_argument('--obfuscate', action='store_true', help='Enable payload obfuscation')
    parser.add_argument('--verbose', action='store_true', help='Include response snippet')
    parser.add_argument('--proxies', default=config.get('proxies', ''), help='File containing list of proxies')
    parser.add_argument('-A', '--header', action='append', default=config.get('headers', []), help='Custom header (e.g. -A "X-Token:123")', dest='custom_headers')
    # Fix #14: --method flag
    parser.add_argument('--method', default='GET', choices=['GET', 'POST', 'PUT', 'DELETE', 'HEAD', 'OPTIONS', 'PATCH'], help='HTTP method to use (default: GET)')
    # Fix #15: --filter-code flag
    parser.add_argument('--filter-code', type=int, action='append', dest='filter_codes', default=[404], help='Status codes to suppress from output (default: 404)')
    # Issue 1: Size filtering
    parser.add_argument('--filter-size', type=int, action='append', dest='filter_sizes',
                        default=config.get('filter_sizes', []),
                        help='Hide responses of this exact byte length (can repeat)')
    parser.add_argument('--match-size', type=int, action='append', dest='match_sizes',
                        default=config.get('match_sizes', []),
                        help='Only show responses of this exact byte length (can repeat)')
    # Issue 2: Word count filtering
    parser.add_argument('--filter-words', type=int, action='append', dest='filter_words',
                        default=config.get('filter_words', []),
                        help='Hide responses with this word count (can repeat)')
    parser.add_argument('--match-words', type=int, action='append', dest='match_words',
                        default=config.get('match_words', []),
                        help='Only show responses with this word count (can repeat)')
    # Issue 3: Response body matching
    parser.add_argument('--match-string', type=str, dest='match_string', default=None,
                        help='Only show responses containing this string')
    parser.add_argument('--match-regex', type=str, dest='match_regex', default=None,
                        help='Only show responses matching this regex pattern')
    # Issue 4: POST body fuzzing
    parser.add_argument('--data', type=str, default=None,
                        help='POST body data with FUZZ keyword (e.g. --data "username=FUZZ&password=test")')
    # Issue 6: Resume support
    parser.add_argument('--resume', action='store_true',
                        help='Resume from existing output file, skipping already-tested URLs')
    # Issue 7: Output formats
    parser.add_argument('--format', choices=['jsonl', 'csv', 'html'], default='jsonl',
                        help='Output format: jsonl (default), csv, or html')
    # Issue 8: Recursive mode
    parser.add_argument('--recursive', action='store_true', help='Automatically fuzz discovered directories')
    parser.add_argument('--recursion-depth', type=int, default=2,
                        help='Maximum recursion depth (default: 2)')
    args = parser.parse_args()

    # Handle wordlist default for backward compatibility
    if args.wordlists is None:
        args.wordlists = [config.get('wordlist', 'wordlists/common.txt')]

    # Fix #6: FUZZ keyword validation (supports FUZZ, FUZZ1, FUZZ2, etc.)
    combined = args.url + (args.data or '')
    if "FUZZ" not in combined:
        print("[bold red]Error: FUZZ keyword must appear in the URL or --data body[/bold red]")
        sys.exit(1)

    # Warn about --data with GET
    if args.data and args.method == 'GET':
        print("[yellow]Warning: --data provided with GET method. Consider using --method POST.[/yellow]")

    asyncio.run(
        fr3ki_fuzzer(
            args.url, args.wordlists, args.threads, args.verbose,
            args.output, args.obfuscate, args.rate, args.cooldown, args.debug,
            args.proxies, args.custom_headers, args.method, args.filter_codes,
            args.filter_sizes, args.match_sizes, args.filter_words, args.match_words,
            args.match_string, args.match_regex, args.data, args.resume, args.format,
            args.recursive, args.recursion_depth
        )
    )

if __name__ == "__main__":
    main()
