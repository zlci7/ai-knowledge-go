package service

import "testing"

// TestSplitByRuneLength 验证文本可被稳定切分为多个非空分块。
func TestSplitByRuneLength(t *testing.T) {
	text := "这是一个用于测试分块逻辑的长文本。我们希望它能被切成多个块，并且每个块长度合理。"
	chunks := splitByRuneLength(text, 12, 3)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if chunk == "" {
			t.Fatalf("chunk %d should not be empty", i)
		}
	}
}

// TestCleanTags 验证标签清洗会去除空白并按顺序去重。
func TestCleanTags(t *testing.T) {
	tags := []string{" 部署 ", "K8s", "", "部署", "  "}
	got := cleanTags(tags)
	if len(got) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(got))
	}
	if got[0] != "部署" || got[1] != "K8s" {
		t.Fatalf("unexpected tags: %#v", got)
	}
}

// TestIsAllowedExt 验证扩展名白名单判断逻辑。
func TestIsAllowedExt(t *testing.T) {
	if !isAllowedExt("pdf") {
		t.Fatal("pdf should be allowed")
	}
	if isAllowedExt("exe") {
		t.Fatal("exe should not be allowed")
	}
}
