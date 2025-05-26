#!/usr/bin/env python3

import sys
import os
import argparse
import asyncio
import httpx
import random
import time
import json
import base64
import urllib.parse
import yaml

# Venv check & rich check (prints instructions if missing)
def _check_venv():
    if sys.prefix == sys.base_prefix:
        print("\n[bold yellow]⚠️  You are NOT in a virtual environment.[/bold yellow]")
        print("For best results, run:")
        print("    python3 -m venv fr3ki_env")
        print("    source fr3ki_env/bin/activate")
        print("    pip install -r requirements.txt\n")
        print("Or install dependencies with:")
        print("    pip install rich httpx pyyaml\n")
        print("Then re-run this script!")
        sys.exit(1)
try:
    from rich import print
    from rich.progress import Progress
except ImportError:
    print("You need the 'rich' library for color output.\nInstall with: pip install rich\n")
    sys.exit(1)
_check_venv()

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

# Config load (YAML fallback)
def load_config(config_file):
    try:
        with open(config_file, 'r') as file:
            config = yaml.safe_load(file)
            return config if config else {}
    except FileNotFoundError:
        print("[yellow]Config file not found. Using default settings.[/yellow]")
        return {}

# Chunk utility
def chunked(lst, n):
    for i in range(0, len(lst), n):
        yield lst[i:i + n]

# Obfuscation (toggleable)
def obfuscate_payload(word):
    payloads = [word]
    # URL, Base64, Hex, Slash-encode, Unicode, Double URL
    payloads.append(urllib.parse.quote(word))
    payloads.append(base64.b64encode(word.encode()).decode())
    payloads.append(''.join(['%{:02x}'.format(ord(c)) for c in word]))
    payloads.append(word.replace('/', '%2F').replace(':', '%3A'))
    payloads.append(''.join(['\\u{:04x}'.format(ord(c)) for c in word]))
    payloads.append(urllib.parse.quote(urllib.parse.quote(word)))
    return list(set(payloads))

# Incremental save (silent)
def incremental_save(entry, filename):
    with open(filename, 'a') as f:
        f.write(json.dumps(entry) + "\n")

# Proxy loader
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

# User-Agent and headers
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
        # Custom headers override built-ins
        for h in custom_headers:
            parts = h.split(":", 1)
            if len(parts) == 2:
                k, v = parts
                headers[k.strip()] = v.strip()
    return headers

# The fuzzer itself (feature complete)
async def fr3ki_fuzzer(
    base_url, wordlist, threads, verbose, output, obfuscate, rate,
    cooldown, debug, proxy_file, custom_headers
):
    proxies = load_proxies(proxy_file) if proxy_file else []
    with open(wordlist) as f:
        words = [line.strip() for line in f if line.strip()]
    sem = asyncio.Semaphore(threads)
    results = []

    async with httpx.AsyncClient(timeout=10, follow_redirects=True) as client:
        with Progress() as progress:
            task = progress.add_task("[cyan]Fuzzing with fr3ki...[/cyan]", total=len(words))
            for chunk in chunked(words, threads):
                tasks = []
                for word in chunk:
                    payloads = obfuscate_payload(word) if obfuscate else [word]
                    for payload in payloads:
                        url = base_url.replace("FUZZ", payload)
                        proxy = get_random_proxy(proxies)
                        headers = generate_headers(custom_headers)
                        req_kwargs = {'headers': headers}
                        if proxy:
                            req_kwargs['proxies'] = {"http://": proxy, "https://": proxy}
                        async def fetch_url(url, req_kwargs, word=word):
                            async with sem:
                                if rate > 0:
                                    await asyncio.sleep(1/rate)
                                try:
                                    resp = await client.get(url, **req_kwargs)
                                    entry = {
                                        "url": url,
                                        "status_code": resp.status_code,
                                        "length": len(resp.content)
                                    }
                                    if verbose:
                                        entry["snippet"] = resp.text[:200]
                                    if debug or (resp.status_code in [200,201,202,204] or len(resp.content) > 1000 or 'admin' in url or 'secure' in url):
                                        incremental_save(entry, output)
                                    # Adaptive 429
                                    if resp.status_code == 429:
                                        # Support Retry-After
                                        retry_after = resp.headers.get('Retry-After')
                                        cooldown_time = int(retry_after) if retry_after and retry_after.isdigit() else cooldown
                                        print(f"[yellow]429 received, cooling down for {cooldown_time} seconds.[/yellow]")
                                        await asyncio.sleep(cooldown_time)
                                    if resp.status_code in [200,201,202,204]:
                                        print(f"[green]{url} [{resp.status_code}][/green]")
                                    elif resp.status_code == 403:
                                        print(f"[yellow]{url} [{resp.status_code} Forbidden][/yellow]")
                                    elif resp.status_code == 404:
                                        print(f"[cyan]{url} [{resp.status_code} Not Found][/cyan]")
                                    else:
                                        print(f"[red]{url} [{resp.status_code}][/red]")
                                except Exception as e:
                                    print(f"[red]Error with {url}: {e}[/red]")
                        tasks += [fetch_url(url, req_kwargs) for payload in payloads]
                await asyncio.gather(*tasks)
                progress.update(task, advance=len(chunk))
                # Add random interval for anti-WAF
                await asyncio.sleep(random.uniform(0.5, 2.0))

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
    args = parser.parse_args()

    asyncio.run(
        fr3ki_fuzzer(
            args.url, args.wordlist, args.threads, args.verbose,
            args.output, args.obfuscate, args.rate, args.cooldown, args.debug,
            args.proxies, args.custom_headers
        )
    )

if __name__ == "__main__":
    main()
