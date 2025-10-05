# GoLinkfinder-
Implementation of the original LinkFinder utility in Go.

## Usage

```bash
go run . -i <target> [-o output.html] [--regex <filter>] [--domain] [--cookies <cookie-string>] [--timeout <seconds>]
```

Use `-o cli` to print matches to stdout. The program accepts the same kinds of inputs as the Python version, including URLs, local files, wildcards and Burp XML exports (`-b`).
