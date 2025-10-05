package model

// Target represents a resource to fetch and analyse.
type Target struct {
	URL        string
	Content    string
	Prefetched bool
}

// Endpoint represents an extracted endpoint and its context.
type Endpoint struct {
	Link    string
	Context string
	Line    int
}
