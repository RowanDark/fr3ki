# fr3ki

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/T6T61FKEIB)

> **This is an in-progress Go port of fr3ki.**
> The original Python implementation lives in `fr3ki.py` and remains functional.
> This Go version is being developed for consistency with the rest of the Rosec toolchain.
> No fuzzing logic has been ported yet — current work is scaffolding only.

---

## Building (Go)

```bash
make build
./bin/fr3ki --version
```

Requires Go 1.21+.

---

## Original Python tool

**fr3ki** is an advanced asynchronous fuzzer designed for bug bounty hunters, penetration testers, and red teamers. It features high concurrency, payload obfuscation, proxy rotation, adaptive throttling, and much more—all in a single extensible Python tool.

**NOTE** Only use this on programs and applications that you are authorized to perform research and testing on! Failure to do so is considered illegal in most jurisdictions, and you do so at your own risk!

### Python installation

```bash
git clone https://github.com/RowanDark/fr3ki.git
cd fr3ki
python3 -m venv fr3ki_env
source fr3ki_env/bin/activate
pip install -r requirements.txt
```

### Python usage

```bash
python3 fr3ki.py -u https://target.com/FUZZ -w wordlists/common.txt -o results.json
```

See the full flag reference and examples in the [original README history](https://github.com/RowanDark/fr3ki/blob/main/README.md).

---

## License

MIT License — see LICENSE file.

Developed with 🐺 by RowanDark, 2025.
