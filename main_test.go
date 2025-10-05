package main

import (
	"context"
	"sync"
	"testing"

	"github.com/example/GoLinkfinderEVO/internal/config"
	"github.com/example/GoLinkfinderEVO/internal/model"
)

func TestProcessDomainMaxDepth(t *testing.T) {
	cfg := config.Config{MaxDepth: 1}
	visited := newVisitedSet()

	endpoints := []model.Endpoint{
		{Link: "https://example.com/a.js"},
		{Link: "https://example.com/b.js"},
	}

	var mu sync.Mutex
	var tasks []resourceTask
	enqueue := func(task resourceTask) {
		mu.Lock()
		defer mu.Unlock()
		tasks = append(tasks, task)
	}

	processDomain(context.Background(), cfg, "https://example.com/index.js", endpoints, visited, enqueue, cfg.MaxDepth)

	if len(tasks) != len(endpoints) {
		t.Fatalf("expected %d tasks, got %d", len(endpoints), len(tasks))
	}

	for _, task := range tasks {
		if task.depth != 0 {
			t.Fatalf("expected depth 0 for enqueued task, got %d", task.depth)
		}
	}

	var next []resourceTask
	for _, task := range tasks {
		deeperEndpoints := []model.Endpoint{{Link: task.target.URL + "-deeper.js"}}
		processDomain(context.Background(), cfg, task.target.URL, deeperEndpoints, visited, func(task resourceTask) {
			next = append(next, task)
		}, task.depth)
	}

	if len(next) != 0 {
		t.Fatalf("expected no tasks due to depth limit, got %d", len(next))
	}
}

func TestProcessDomainScopeFiltering(t *testing.T) {
	base := "https://example.com/app/index.js"
	endpoints := []model.Endpoint{
		{Link: "https://example.com/static/app.js"},
		{Link: "https://cdn.example.com/bundle.js"},
		{Link: "https://malicious.net/exfil.js"},
		{Link: "/local.js"},
	}

	tests := []struct {
		name    string
		scope   string
		include bool
		want    []string
	}{
		{
			name:    "exact host only",
			scope:   "example.com",
			include: false,
			want: []string{
				"https://example.com/static/app.js",
				"https://example.com/local.js",
			},
		},
		{
			name:    "include subdomains",
			scope:   "example.com",
			include: true,
			want: []string{
				"https://example.com/static/app.js",
				"https://example.com/local.js",
				"https://cdn.example.com/bundle.js",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{Scope: tt.scope, ScopeIncludeSubdomains: tt.include}
			visited := newVisitedSet()
			visited.Add(base)

			var mu sync.Mutex
			var tasks []resourceTask
			enqueue := func(task resourceTask) {
				mu.Lock()
				defer mu.Unlock()
				tasks = append(tasks, task)
			}

			processDomain(context.Background(), cfg, base, endpoints, visited, enqueue, 0)

			if len(tasks) != len(tt.want) {
				t.Fatalf("expected %d tasks, got %d", len(tt.want), len(tasks))
			}

			expected := make(map[string]struct{}, len(tt.want))
			for _, url := range tt.want {
				expected[url] = struct{}{}
			}

			for _, task := range tasks {
				if _, ok := expected[task.target.URL]; !ok {
					t.Fatalf("unexpected task for %s", task.target.URL)
				}
				delete(expected, task.target.URL)
			}

			if len(expected) != 0 {
				t.Fatalf("missing tasks for URLs: %v", keys(expected))
			}
		})
	}
}

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
