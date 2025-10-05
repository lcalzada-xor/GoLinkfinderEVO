# GoLinkfinder-
Implementation of the original LinkFinder utility in Go.

## Usage

```bash
go run . -i <target> [-o output.html] [--regex <filter>] [--domain] [--scope <domain>] [--cookies <cookie-string>] [--proxy <url>] [--insecure] [--timeout <seconds>]
```

The tool now prints matches to stdout in raw format by default. Provide `-o <file.html>` if you want to save the HTML report instead, or `--raw <file>` to export a machine-friendly plaintext list. The program accepts the same kinds of inputs as the Python version, including URLs, local files, wildcards and Burp XML exports (`-b`).

When working behind a proxy, pass `--proxy http://127.0.0.1:8080` (or the appropriate scheme and address) to forward all outbound requests through it. For targets that use self-signed certificates you can opt into skipping TLS verification with `--insecure`; this should only be used in trusted environments such as local testing setups.

The HTML report template is now embedded within the binary, so you no longer need to keep `template.html` alongside the executable when running the tool.
