# GoLinkfinder-
Implementation of the original LinkFinder utility in Go.

## Usage

```bash
go run . -i <target> [-o output.html] [--regex <filter>] [--domain] [--scope <domain>] [--cookies <cookie-string>] [--timeout <seconds>] [--workers <n>]
```

The tool now prints matches to stdout in raw format by default. Provide `-o <file.html>` if you want to save the HTML report instead, or `--raw <file>` to export a machine-friendly plaintext list. The program accepts the same kinds of inputs as the Python version, including URLs, local files, wildcards and Burp XML exports (`-b`).

The HTML report template is now embedded within the binary, so you no longer need to keep `template.html` alongside the executable when running the tool.

### Concurrency control

Use `--workers` to limit how many resources are fetched in parallel. The flag defaults to the number of logical CPUs available, but you can lower it when targeting rate-limited endpoints or increase it when you need faster crawling on permissive targets.
