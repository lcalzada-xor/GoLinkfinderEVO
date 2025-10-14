package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/browser"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/config"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/model"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/network"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/parser"
)

type payload struct {
	HTML string `json:"html"`
}

func TestRenderModeExtractsDynamicEndpoints(t *testing.T) {
	if !browser.IsAvailable() {
		t.Skip("no compatible browser available for render tests")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><html><head><title>render</title></head><body><script src="/writer.js"></script><script src="/fetcher.js"></script></body></html>`)
	})
	mux.HandleFunc("/writer.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprint(w, `document.write('<a id="docwrite" href="/api/from-document-write">doc</a>');`)
	})
	mux.HandleFunc("/fetcher.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprint(w, `fetch('/payload').then(resp => resp.json()).then(data => { const host = document.createElement('div'); host.innerHTML = data.html; document.body.appendChild(host); });`)
	})
	mux.HandleFunc("/payload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload{HTML: `<a id="fetched" href="/api/from-fetch">fetch</a>`})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	baseURL := server.URL + "/"
	regex := parser.EndpointRegex()

	ctx := context.Background()

	staticCfg := config.Config{Timeout: 5 * time.Second}
	staticHTML, err := network.Fetch(ctx, baseURL, staticCfg)
	if err != nil {
		t.Fatalf("static fetch returned error: %v", err)
	}
	staticEndpoints := collectLinks(parser.FindEndpoints(staticHTML, regex, true, nil, true))

	renderCfg := config.Config{Timeout: 10 * time.Second, Render: true}
	renderedHTML, err := network.Fetch(ctx, baseURL, renderCfg)
	if err != nil {
		t.Fatalf("rendered fetch returned error: %v", err)
	}
	renderedEndpoints := collectLinks(parser.FindEndpoints(renderedHTML, regex, true, nil, true))

	if renderedEndpoints["/api/from-document-write"] == 0 {
		t.Fatalf("rendered fetch did not contain document.write endpoint; got %v", renderedEndpoints)
	}
	if renderedEndpoints["/api/from-fetch"] == 0 {
		t.Fatalf("rendered fetch did not contain fetch endpoint; got %v", renderedEndpoints)
	}

	if _, ok := staticEndpoints["/api/from-document-write"]; ok {
		t.Fatalf("non rendered fetch unexpectedly contained document.write endpoint")
	}
	if _, ok := staticEndpoints["/api/from-fetch"]; ok {
		t.Fatalf("non rendered fetch unexpectedly contained fetch endpoint")
	}
}

func collectLinks(endpoints []model.Endpoint) map[string]int {
	out := make(map[string]int, len(endpoints))
	for _, ep := range endpoints {
		out[ep.Link]++
	}
	return out
}
