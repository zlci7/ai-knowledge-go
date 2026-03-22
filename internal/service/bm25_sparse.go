package service

import (
	"ai-knowledge-go/internal/repository/vector"
	"hash/fnv"
	"sort"
	"strings"
	"sync"

	"github.com/go-ego/gse"
)

const (
	bm25K1        = 1.2
	bm25B         = 0.75
	bm25AvgDocLen = 32.0
)

var (
	bm25Seg  gse.Segmenter
	bm25Once sync.Once
)

func ensureBM25Segmenter() {
	bm25Once.Do(func() {
		// gse 默认词典，支持中文检索分词。
		bm25Seg.LoadDict()
	})
}

// encodeBM25Sparse 将文本编码为 BM25 风格 sparse 向量（仅依赖当前文本统计）。
func encodeBM25Sparse(text string) vector.SparseVector {
	ensureBM25Segmenter()
	tokens := bm25Seg.Cut(strings.TrimSpace(text))
	if len(tokens) == 0 {
		return vector.SparseVector{}
	}

	tf := make(map[string]float64, len(tokens))
	docLen := 0.0
	for _, raw := range tokens {
		token := normalizeToken(raw)
		if token == "" {
			continue
		}
		tf[token] += 1
		docLen++
	}
	if docLen == 0 {
		return vector.SparseVector{}
	}

	scoreByIndex := make(map[uint32]float64, len(tf))
	for token, freq := range tf {
		tfNorm := (freq * (bm25K1 + 1)) / (freq + bm25K1*(1-bm25B+bm25B*(docLen/bm25AvgDocLen)))
		idx := hashToken(token)
		scoreByIndex[idx] += tfNorm
	}

	indices := make([]uint32, 0, len(scoreByIndex))
	for idx := range scoreByIndex {
		indices = append(indices, idx)
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })

	values := make([]float32, 0, len(indices))
	for _, idx := range indices {
		values = append(values, float32(scoreByIndex[idx]))
	}

	return vector.SparseVector{
		Indices: indices,
		Values:  values,
	}
}

func normalizeToken(token string) string {
	token = strings.TrimSpace(strings.ToLower(token))
	if token == "" {
		return ""
	}
	return strings.Trim(token, ",.!?;:()[]{}\"'`，。！？；：（）【】")
}

func hashToken(token string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(token))
	return h.Sum32()
}
