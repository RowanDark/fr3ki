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

def incremental_save(entry, filename):
    with open(filename, 'a') as f:
        f.write(json.dumps(entry) + "\n")

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
    base_url, wordlist, threads, verbose, output, obfuscate, rate,
    cooldown, debug, proxy_file, custom_headers, method, filter_codes
):
    proxies = load_proxies(proxy_file) if proxy_file else []
    with open(wordlist) as f:
        words = [line.strip() for line in f if line.strip()]
    sem = asyncio.Semaphore(threads)
    # Fix #4: instantiate global rate limiter
    limiter = RateLimiter(rate)
    # Fix #12: counters for summary
    counter = [0, 0]  # [total_requests, interesting_results]

    # Shared client for requests without a proxy (preserves connection pooling)
    shared_client = httpx.AsyncClient(timeout=10, follow_redirects=True)

    async def fetch_url(url, proxy, headers):
        async with sem:
            # Fix #4: acquire global rate limiter
            await limiter.acquire()
            # Fix #10: per-request jitter for evasion
            await asyncio.sleep(random.uniform(0.05, 0.3))
            try:
                if proxy:
                    proxy_map = {"http://": proxy, "https://": proxy}
                    async with httpx.AsyncClient(timeout=10, follow_redirects=True, proxies=proxy_map) as client:
                        resp = await client.request(method, url, headers=headers)
                else:
                    resp = await shared_client.request(method, url, headers=headers)
                counter[0] += 1
                entry = {
                    "url": url,
                    "status_code": resp.status_code,
                    "length": len(resp.content)
                }
                if verbose:
                    entry["snippet"] = resp.text[:200]
                # Fix #9: improved interesting filter
                if debug or resp.status_code in INTERESTING_CODES or len(resp.content) > 500:
                    incremental_save(entry, output)
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
            except httpx.TimeoutException:
                print(f"[yellow]Timeout: {url}[/yellow]")
            except httpx.ConnectError:
                print(f"[red]Connection error: {url}[/red]")
            except Exception as e:
                print(f"[red]Error with {url}: {e}[/red]")

    with Progress() as progress:
        task = progress.add_task("[cyan]Fuzzing with fr3ki...[/cyan]", total=len(words))
        for chunk in chunked(words, threads):
            tasks = []
            # Fix #1: single payload iteration, no double loop
            for word in chunk:
                payloads = obfuscate_payload(word) if obfuscate else [word]
                for payload in payloads:
                    url = base_url.replace("FUZZ", payload)
                    proxy = get_random_proxy(proxies)
                    headers = generate_headers(custom_headers)
                    tasks.append(fetch_url(url, proxy, headers))
            await asyncio.gather(*tasks)
            progress.update(task, advance=len(chunk))
            # Fix #10: removed chunk-level sleep (jitter is per-request now)

    await shared_client.aclose()

    # Fix #12: output summary
    print(f"\n[bold green]✓ Fuzzing complete.[/bold green] {counter[0]} requests sent. Results saved to [cyan]{output}[/cyan]")


def main():
    print_banner()
    config = load_config('fr3ki_config.yaml')
    parser = argparse.ArgumentParser(description="fr3ki - Advanced Fuzzer by RowanDark")
    parser.add_argument('-u', '--url', required=True, help='Target URL with FUZZ keyword')
    parser.add_argument('-w', '--wordlist', default=config.get('wordlist', 'wordlists/common.txt'), help='Wordlist file')
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
    args = parser.parse_args()

    # Fix #6: FUZZ keyword validation
    if "FUZZ" not in args.url:
        print("[bold red]Error: URL must contain the FUZZ keyword (e.g. https://target.com/FUZZ)[/bold red]")
        sys.exit(1)

    asyncio.run(
        fr3ki_fuzzer(
            args.url, args.wordlist, args.threads, args.verbose,
            args.output, args.obfuscate, args.rate, args.cooldown, args.debug,
            args.proxies, args.custom_headers, args.method, args.filter_codes
        )
    )

if __name__ == "__main__":
    main()
