package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/example/GoLinkfinderEVO/internal/config"
	"github.com/example/GoLinkfinderEVO/internal/model"
	"github.com/example/GoLinkfinderEVO/internal/output"
	"github.com/example/GoLinkfinderEVO/internal/parser"
)

func TestProcessDomainHonorsMaxDepth(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		requests = make(map[string]int)
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests[r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/level1.js":
			fmt.Fprint(w, `var next = "/level2.js";`)
		case "/level2.js":
			fmt.Fprint(w, `var next = "/level3.js";`)
		case "/level3.js":
			fmt.Fprint(w, `console.log("too deep");`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.Config{Timeout: time.Second, MaxDepth: 2}
	endpoints := []model.Endpoint{{Link: "/level1.js"}}
	regex := parser.EndpointRegex()
	visited := make(map[string]struct{})
	var reports []output.ResourceReport
	var builder strings.Builder

	processDomain(cfg, server.URL+"/index.html", endpoints, regex, nil, output.ModeHTML, &builder, &reports, visited, cfg.MaxDepth)

	if got := requests["/level1.js"]; got == 0 {
		t.Fatalf("expected /level1.js to be fetched, got %d", got)
	}
	if got := requests["/level2.js"]; got == 0 {
		t.Fatalf("expected /level2.js to be fetched, got %d", got)
	}
	if got := requests["/level3.js"]; got != 0 {
		t.Fatalf("expected recursion to stop before /level3.js, got %d fetches", got)
	}
}
