package service

import (
	"ai-knowledge-go/internal/repository/vector"
	"reflect"
	"strings"
	"testing"
)

func TestFormatKnowledgeRAGSystemPromptEmpty(t *testing.T) {
	if got := formatKnowledgeRAGSystemPrompt(nil); got != "" {
		t.Fatalf("expected empty prompt, got %q", got)
	}
}

func TestFormatKnowledgeRAGSystemPromptSingle(t *testing.T) {
	chunks := []vector.KnowledgeSearchResult{
		{
			DocID:   1,
			DocName: "deploy.md",
			Content: "Use rolling update for production releases.",
			Score:   0.9,
		},
	}
	got := formatKnowledgeRAGSystemPrompt(chunks)
	if !strings.Contains(got, "[知识库召回-仅在相关时使用]") {
		t.Fatalf("unexpected prompt header: %q", got)
	}
	if !strings.Contains(got, "1. [deploy.md] Use rolling update for production releases.") {
		t.Fatalf("unexpected prompt content: %q", got)
	}
}

func TestRetrieveKnowledgePromptTruncation(t *testing.T) {
	longContent := strings.Repeat("x", ragChunkMaxChars+80)
	chunks := []vector.KnowledgeSearchResult{
		{
			DocID:   10,
			DocName: "runbook",
			Content: truncateRunes(longContent, ragChunkMaxChars),
		},
	}
	got := formatKnowledgeRAGSystemPrompt(chunks)
	if !strings.Contains(got, "...") {
		t.Fatalf("expected truncated content with ellipsis, got %q", got)
	}
	if len(got) > ragPromptMaxChars {
		t.Fatalf("prompt too long: %d", len(got))
	}
}

func TestFormatKnowledgeRAGSystemPromptMultiLimit(t *testing.T) {
	chunks := make([]vector.KnowledgeSearchResult, 0, 20)
	for i := 0; i < 20; i++ {
		chunks = append(chunks, vector.KnowledgeSearchResult{
			DocID:   uint64(i + 1),
			DocName: "doc",
			Content: strings.Repeat("a", 180),
		})
	}
	got := formatKnowledgeRAGSystemPrompt(chunks)
	if len(got) > ragPromptMaxChars {
		t.Fatalf("prompt exceeds max chars: %d", len(got))
	}
}

func TestEncodeBM25SparseStable(t *testing.T) {
	text := "K8s 部署 CrashLoopBackOff 排查"
	v1 := encodeBM25Sparse(text)
	v2 := encodeBM25Sparse(text)
	if len(v1.Indices) == 0 || len(v1.Values) == 0 {
		t.Fatal("expected non-empty sparse vector")
	}
	if !reflect.DeepEqual(v1, v2) {
		t.Fatalf("expected deterministic sparse encoding, got %#v vs %#v", v1, v2)
	}
	for i := 1; i < len(v1.Indices); i++ {
		if v1.Indices[i] < v1.Indices[i-1] {
			t.Fatalf("indices should be sorted: %v", v1.Indices)
		}
	}
}

func TestMergeKnowledgeHitsRRF(t *testing.T) {
	dense := []vector.KnowledgeSearchResult{
		{PointID: 1, DocID: 1, Content: "a"},
		{PointID: 2, DocID: 2, Content: "b"},
	}
	sparse := []vector.KnowledgeSearchResult{
		{PointID: 2, DocID: 2, Content: "b2"},
		{PointID: 3, DocID: 3, Content: "c"},
	}

	merged := mergeKnowledgeHitsRRF(dense, sparse, ragRRFK)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged results, got %d", len(merged))
	}
	if merged[0].PointID != 2 {
		t.Fatalf("expected point 2 to rank first, got %d", merged[0].PointID)
	}
}

func TestMergeKnowledgeHitsRRFDedupByPointID(t *testing.T) {
	dense := []vector.KnowledgeSearchResult{
		{PointID: 7, DocID: 17, Content: "dense text", Source: "dense"},
	}
	sparse := []vector.KnowledgeSearchResult{
		{PointID: 7, DocID: 17, Content: "sparse text", Source: "sparse"},
	}

	merged := mergeKnowledgeHitsRRF(dense, sparse, ragRRFK)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged result, got %d", len(merged))
	}
	if merged[0].PointID != 7 {
		t.Fatalf("unexpected point id: %d", merged[0].PointID)
	}
	if merged[0].Source != "hybrid" {
		t.Fatalf("expected hybrid source, got %q", merged[0].Source)
	}
}
