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
	denseName  string
	sparseName string
	sparseOn   bool
}

type MemorySearchResult struct {
	MemoryID uint64
	Score    float64
	Content  string
	Category string
}

type KnowledgeSearchResult struct {
	PointID  uint64
	DocID    uint64
	Score    float64
	RRFScore float64
	Content  string
	DocName  string
	Source   string
}

type KnowledgeChunkPoint struct {
	ID         uint64
	Vector     []float32
	Sparse     SparseVector
	Content    string
	KBID       uint64
	DocID      uint64
	DocName    string
	DocType    string
	Project    string
	Tags       []string
	ChunkIndex int
}

type SparseVector struct {
	Indices []uint32
	Values  []float32
}
