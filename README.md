# GoLinkFinder EVO

![Go version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white) ![Security](https://img.shields.io/badge/focus-application%20security-red) ![License](https://img.shields.io/badge/license-MIT-green)

**GoLinkFinder EVO** is a fast, batteries-included rewrite of the classic [LinkFinder](https://github.com/GerbenJavado/LinkFinder) reconnaissance utility for security researchers, bug bounty hunters, and penetration testers. Built in Go for speed and portability, it discovers JavaScript endpoints, API calls, and other juicy URLs hidden in web applications so you can map attack surfaces in seconds.

---

## Table of contents

- [Why GoLinkFinder EVO?](#why-golinkfinder-evo)
- [Key features](#key-features)
- [Getting started](#getting-started)
  - [Quick install](#quick-install)
  - [Basic usage](#basic-usage)
  - [Advanced examples](#advanced-examples)
- [Flags reference](#flags-reference)
- [Performance tuning](#performance-tuning)
- [Tips for better recon results](#tips-for-better-recon-results)
- [Contributing](#contributing)
- [Roadmap](#roadmap)
- [Community & support](#community--support)

---

## Why GoLinkFinder EVO?

* **Zero dependencies:** Ship a single binary that works on macOS, Linux, and Windows without Python runtimes or virtual environments.
* **Fast enumeration:** Native Go concurrency lets you crawl large targets quickly while keeping memory usage low.
* **Battle-tested workflow:** Supports the same input sources and regex patterns that made the original LinkFinder a staple in JavaScript recon.

Use GoLinkFinder EVO to supercharge your bug bounty methodology, automate URL discovery inside CI pipelines, or supplement tools like Burp Suite, httpx, gau, hakrawler, and Katana.

## Key features

- 🔍 **Smart pattern matching** – Extract JavaScript endpoints, REST routes, AWS/GCP URLs, JWTs, keys, and more with customizable regex filters.
- 📄 **Flexible outputs** – Stream matches to stdout, generate HTML reports for presentations, export plain text with `--raw`, or produce machine-readable JSON for integrations.
- 🧰 **gf rule integration** – Run your favourite [tomnomnom/gf](https://github.com/tomnomnom/gf) patterns against discovered endpoints and export structured findings automatically.
- 🌐 **Scope-aware crawling** – Constrain discovery to specific domains, respect scopes, and feed data from live URLs, local JS bundles, or Burp XML exports (`-b`).
- 🔒 **Proxy & TLS control** – Route traffic through Burp/ZAP with `--proxy` or skip verification for lab environments via `--insecure`.
- ⚙️ **Parallel workers** – Configure worker pools with `--workers` to balance speed, rate limits, and stealth.

## Getting started

### Quick install

```bash
git clone https://github.com/lcalzada-xor/GoLinkfinderEVO.git
cd GoLinkfinderEVO
go build -o golinkfinder
# Optional: add to PATH
sudo mv golinkfinder /usr/local/bin/
```

Or run directly with `go run .` if you prefer not to build a binary.

### Basic usage

```bash
go run . -i https://target.com --output html=report.html
```

This command crawls `https://target.com`, prints discovered endpoints to stdout, and saves an interactive HTML report to `report.html`.

### Advanced examples

```bash
# Scan a local JavaScript bundle and filter for API paths only
go run . -i ./static/app.js --regex api --raw api-endpoints.txt

# Enumerate a target through Burp, include subdomains, and output JSON for further scripting
go run . -i https://scope.example --scope example --scope-include-subdomains --proxy http://127.0.0.1:8080 --output json=findings.json

# Crawl a domain and emit CLI, HTML, and JSON outputs simultaneously
go run . -i https://target.com --output cli,html=report.html,json=findings.json

# Import historical data from a Burp Suite XML export
go run . -b ./traffic-export.xml --workers 20
```

### gf integration

Place your gf JSON definitions inside `~/.gf` (the same convention used by the original tool). Then pass either a comma-separated list of rule names or `all` to execute every JSON file:

```bash
# Run specific gf rules and generate gf.txt / gf.json with matches
go run . -i https://target.com --gf jwt,urls

# Execute every rule found inside ~/.gf
go run . -i https://target.com --gf all
```

The generated `gf.txt` and `gf.json` files include the resource path, line number, matching evidence, and the rule responsible for each finding.

## Flags reference

| Flag | Description |
| ---- | ----------- |
| `-i, --input` | URL, file, glob pattern, or directory to scan. |
| `-b, --burp` | Parse Burp Suite XML exports as input. |
| `-o, --output` | Configure outputs. Accepts values like `cli`, `html=report.html`, `json=findings.json`, or `raw=endpoints.txt`. Repeat or comma-separate to combine formats. |
| `--raw` | Alias for `--output raw=<file>`. |
| `--json` | Alias for `--output json=<file>`. |
| `--regex` | Apply an additional regex filter to matches. |
| `--domain` | Restrict results to the input domain only. |
| `--scope` | Supply a custom allow-list of domains. |
| `--scope-include-subdomains` | Expand `--scope` matches to include subdomains of the provided domain. |
| `--cookies` | Attach cookies to outbound requests. |
| `--proxy` | Proxy all HTTP/S traffic via the given URL. |
| `--insecure` | Skip TLS certificate verification (use with caution). |
| `--timeout` | Configure request timeout in seconds. |
| `--workers` | Tune concurrency level. Defaults to logical CPU count. |
| `--gf` | Execute gf patterns stored in `~/.gf`. Accepts comma-separated rule names or `all` to run every JSON file. Findings are saved to `gf.txt` and `gf.json`. |

## Performance tuning

Leverage Go's concurrency to adapt to target environments:

- Set `--workers` lower (e.g., `--workers 5`) when probing fragile or rate-limited APIs.
- Increase workers (e.g., `--workers 50`) for sprawling JavaScript-heavy single-page applications hosted on CDNs.
- Combine `--timeout` and `--proxy` to stabilize scans routed through intercepting proxies or VPNs.

## Tips for better recon results

- Chain GoLinkFinder EVO with tools like `gau`, `waybackurls`, or `katana` to build comprehensive target lists.
- Use scope flags to keep findings relevant to bug bounty programs and avoid out-of-scope domains.
- Combine `--scope` with `--scope-include-subdomains` when a program allows wildcard coverage beneath a base domain.
- Export JSON and feed it into automation workflows or data lakes for long-term recon tracking.
- Pair with visualization dashboards or Notion/Obsidian notes by saving HTML reports.

## Contributing

Contributions are welcome! If you have ideas for new regex patterns, performance improvements, or UI enhancements for the HTML report, feel free to open an issue or submit a pull request. Please include reproducible steps, sample inputs, and expected outputs when reporting bugs.

Before submitting a PR:

1. Run `go test ./...` to ensure the codebase remains stable.
2. Format your code with `gofmt` or `go fmt ./...`.
3. Document new flags or behavior changes in this README.

## Roadmap

- [ ] Pre-built binaries for common operating systems.
- [ ] Regex presets for popular frameworks (Next.js, Angular, Vue).
- [ ] Optional headless browser integration for dynamic rendering.
- [ ] GitHub Action for automated recon workflows.

## Community & support

- ⭐ **Like the project?** Star the repository to support continued development.
- 🐛 **Found an issue?** [Open a GitHub issue](https://github.com/lcalzada-xor/GoLinkfinderEVO/issues) with details.
- 💬 **Need help?** Start a discussion or reach out on security forums and Discord communities.
- 📰 **Stay updated:** Watch the repo for release announcements and new features.

If GoLinkFinder EVO helps in your bug bounty journey or security assessments, please share it with the community—your feedback and stars keep the project evolving!
