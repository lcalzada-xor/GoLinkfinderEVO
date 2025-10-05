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
