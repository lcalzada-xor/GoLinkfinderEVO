package browser

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/config"
)

const (
	defaultQuietPeriod   = 500 * time.Millisecond
	defaultRenderTimeout = 15 * time.Second
)

// FetchRendered loads the provided URL inside a headless browser and returns the rendered HTML.
// The function respects proxy, cookies and timeout options provided in cfg.
func FetchRendered(ctx context.Context, rawURL string, cfg config.Config) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultRenderTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	allocatorOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("ignore-certificate-errors", cfg.Insecure),
	)

	if cfg.Proxy != "" {
		allocatorOpts = append(allocatorOpts, chromedp.ProxyServer(cfg.Proxy))
	}

	if execPath, ok := findExecPath(); ok {
		allocatorOpts = append(allocatorOpts, chromedp.ExecPath(execPath))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocatorOpts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	headers := network.Headers{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.8",
		"Accept-Encoding": "gzip, deflate, br",
	}

	if cfg.Cookies != "" {
		headers["Cookie"] = cfg.Cookies
	}

	quiet := defaultQuietPeriod
	if timeout > 0 && timeout < quiet {
		quiet = timeout / 2
		if quiet <= 0 {
			quiet = timeout
		}
	}

	tracker := newNetworkIdleTracker(quiet)

	var html string
	err := chromedp.Run(browserCtx,
		tracker.actionAttach(),
		network.Enable(),
		network.SetExtraHTTPHeaders(headers),
		chromedp.Navigate(rawURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		tracker.waitAction(timeout),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)

	return html, err
}

// IsAvailable returns true when a supported Chromium based browser can be located.
func IsAvailable() bool {
	if _, ok := findExecPath(); ok {
		return true
	}

	// chromedp can find the browser automatically even when we don't provide
	// a path. Keep the optimistic behaviour and only report false when the
	// CHROMEDP_EXEC_PATH environment variable explicitly points to an invalid
	// location.
	execPath := strings.TrimSpace(os.Getenv("CHROMEDP_EXEC_PATH"))
	if execPath == "" {
		return false
	}

	if _, err := os.Stat(execPath); err == nil {
		return true
	}

	return false
}

func findExecPath() (string, bool) {
	if env := strings.TrimSpace(os.Getenv("CHROMEDP_EXEC_PATH")); env != "" {
		if stat, err := os.Stat(env); err == nil && !stat.IsDir() {
			return env, true
		}
	}

	names := []string{
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
		"chrome",
		"msedge",
		"microsoft-edge",
	}

	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}

	return "", false
}

type networkIdleTracker struct {
	quiet time.Duration

	mu       sync.Mutex
	inflight int
	idleCh   chan struct{}
}

func newNetworkIdleTracker(quiet time.Duration) *networkIdleTracker {
	if quiet <= 0 {
		quiet = defaultQuietPeriod
	}
	return &networkIdleTracker{quiet: quiet, idleCh: make(chan struct{}, 1)}
}

func (t *networkIdleTracker) actionAttach() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		chromedp.ListenTarget(ctx, func(ev interface{}) {
			switch ev.(type) {
			case *network.EventRequestWillBeSent:
				t.mu.Lock()
				t.inflight++
				t.mu.Unlock()
			case *network.EventLoadingFinished, *network.EventLoadingFailed:
				t.mu.Lock()
				if t.inflight > 0 {
					t.inflight--
				}
				if t.inflight == 0 {
					select {
					case t.idleCh <- struct{}{}:
					default:
					}
				}
				t.mu.Unlock()
			}
		})
		return nil
	})
}

func (t *networkIdleTracker) waitAction(timeout time.Duration) chromedp.Action {
	if timeout <= 0 {
		timeout = defaultRenderTimeout
	}

	return chromedp.ActionFunc(func(ctx context.Context) error {
		quietTimer := time.NewTimer(t.quiet)
		if !quietTimer.Stop() {
			select {
			case <-quietTimer.C:
			default:
			}
		}
		defer quietTimer.Stop()

		timeoutTimer := time.NewTimer(timeout)
		defer timeoutTimer.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timeoutTimer.C:
				return nil
			case <-t.idleCh:
				if !quietTimer.Stop() {
					select {
					case <-quietTimer.C:
					default:
					}
				}
				quietTimer.Reset(t.quiet)
			case <-quietTimer.C:
				t.mu.Lock()
				done := t.inflight == 0
				t.mu.Unlock()
				if done {
					return nil
				}
				quietTimer.Reset(t.quiet)
			}
		}
	})
}
