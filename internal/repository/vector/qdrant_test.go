package vector

import "testing"

func TestParseKnowledgeSearchResultsBasic(t *testing.T) {
	resp := []byte(`{
		"result": [
			{
				"id": 201,
				"score": 0.88,
				"payload": {
					"doc_id": 101,
					"doc_name": "kb.md",
					"content": "deploy with blue-green strategy"
				}
			}
		]
	}`)

	results, err := parseKnowledgeSearchResults(resp, "dense")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].DocID != 101 {
		t.Fatalf("unexpected doc_id: %d", results[0].DocID)
	}
	if results[0].PointID != 201 {
		t.Fatalf("unexpected point_id: %d", results[0].PointID)
	}
	if results[0].DocName != "kb.md" {
		t.Fatalf("unexpected doc_name: %s", results[0].DocName)
	}
	if results[0].Content == "" {
		t.Fatal("content should not be empty")
	}
}

func TestParseKnowledgeSearchResultsMissingPayload(t *testing.T) {
	resp := []byte(`{
		"result": [
			{
				"id": 302,
				"score": 0.51
			}
		]
	}`)

	results, err := parseKnowledgeSearchResults(resp, "dense")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].DocID != 302 {
		t.Fatalf("expected fallback doc_id=302, got %d", results[0].DocID)
	}
	if results[0].PointID != 302 {
		t.Fatalf("expected fallback point_id=302, got %d", results[0].PointID)
	}
}

func TestParseKnowledgeSearchResultsTypeVariants(t *testing.T) {
	resp := []byte(`{
		"result": [
			{
				"id": "999",
				"score": 0.73,
				"payload": {
					"doc_id": "12345",
					"doc_name": 7,
					"content": ["invalid"]
				}
			}
		]
	}`)

	results, err := parseKnowledgeSearchResults(resp, "sparse")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].DocID != 12345 {
		t.Fatalf("unexpected doc_id for string payload: %d", results[0].DocID)
	}
	if results[0].PointID != 999 {
		t.Fatalf("unexpected point_id for string id: %d", results[0].PointID)
	}
	if results[0].DocName != "" {
		t.Fatalf("doc_name should be empty for non-string payload, got %q", results[0].DocName)
	}
	if results[0].Content != "" {
		t.Fatalf("content should be empty for non-string payload, got %q", results[0].Content)
	}
	if results[0].Source != "sparse" {
		t.Fatalf("unexpected source: %q", results[0].Source)
	}
}

func TestParseKnowledgeSearchResultsQueryPointsShape(t *testing.T) {
	resp := []byte(`{
		"result": {
			"points": [
				{
					"id": 88,
					"score": 0.42,
					"payload": {
						"doc_id": 66,
						"doc_name": "ops.md",
						"content": "CrashLoopBackOff 排查"
					}
				}
			]
		}
	}`)

	results, err := parseKnowledgeSearchResults(resp, "sparse")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].PointID != 88 || results[0].DocID != 66 {
		t.Fatalf("unexpected result ids: point=%d doc=%d", results[0].PointID, results[0].DocID)
	}
}
