package vector

import "net/http"

type QdrantRepository struct {
	baseURL    string
	collection string
	client     *http.Client
}

type KnowledgeQdrantRepository struct {
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

type KnowledgeChunkPoint struct {
	ID         uint64
	Vector     []float32
	Content    string
	KBID       uint64
	DocID      uint64
	DocName    string
	DocType    string
	Project    string
	Tags       []string
	ChunkIndex int
}
