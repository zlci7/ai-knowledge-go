package vector

import "net/http"

type QdrantRepository struct {
	baseURL    string
	collection string
	client     *http.Client
}

type MemorySearchResult struct {
	MemoryID uint64
	Score    float64
	Content  string
	Category string
}
