package service

import (
	"ai-knowledge-go/config"
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/api/vo"
	"ai-knowledge-go/internal/llm"
	"ai-knowledge-go/internal/model"
	"ai-knowledge-go/internal/pkg/xerr"
	"ai-knowledge-go/internal/repository/mysql"
	"ai-knowledge-go/internal/repository/vector"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	semanticSimilarityThreshold = 0.72
	minChunkChars               = 120
	maxChunkChars               = 1000
	splitOverlapChars           = 120
	maxErrorMsgLen              = 1000
	defaultListPageSize         = 20
	maxListPageSize             = 100
	chunkIDBits                 = 20
)

var (
	Knowledge = new(KnowledgeService)

	blankLineSplitRegex = regexp.MustCompile(`\n\s*\n+`)
)

type KnowledgeService struct{}

func (s *KnowledgeService) Upload(ctx context.Context, userID, kbID uint64, req dto.DocumentUploadReq, file *multipart.FileHeader) (*vo.DocumentUploadResp, error) {
	if err := s.validateKnowledgeBase(kbID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.DocType) == "" {
		return nil, xerr.NewErrCode(xerr.DOCUMENT_PARAM_ERROR)
	}
	if file == nil {
		return nil, xerr.NewErrCode(xerr.DOCUMENT_PARAM_ERROR)
	}
	if err := s.validateFileHeader(file); err != nil {
		return nil, err
	}

	start := time.Now()
	timeout := time.Duration(config.AppConfig.Knowledge.UploadTimeoutSec) * time.Second
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), ".")
	mimeType := strings.TrimSpace(file.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = mime.TypeByExtension("." + ext)
	}
	tagsJSON, _ := json.Marshal(cleanTags(req.Tags))

	doc := &model.Document{
		KnowledgeBaseID: kbID,
		FileName:        file.Filename,
		FileType:        ext,
		MimeType:        mimeType,
		FileSize:        file.Size,
		DocType:         strings.TrimSpace(req.DocType),
		Project:         strings.TrimSpace(req.Project),
		Tags:            string(tagsJSON),
		Status:          model.DocumentStatusUploading,
		UploadedBy:      userID,
	}
	if err := mysql.Document.Create(opCtx, doc); err != nil {
		return nil, xerr.NewErrCode(xerr.DOCUMENT_UPLOAD_ERROR)
	}

	if err := mysql.Document.UpdateStatus(opCtx, doc.ID, model.DocumentStatusProcessing, 0, ""); err != nil {
		return nil, xerr.NewErrCode(xerr.DOCUMENT_UPLOAD_ERROR)
	}

	fileData, err := readUploadFile(file, config.AppConfig.Knowledge.UploadMaxSizeMB*1024*1024)
	if err != nil {
		s.markFailed(doc.ID, 0, err.Error())
		return nil, xerr.NewErrCode(xerr.DOCUMENT_PARAM_ERROR)
	}

	filePath, err := s.storeFile(kbID, doc.ID, file.Filename, fileData)
	if err != nil {
		s.markFailed(doc.ID, 0, err.Error())
		return nil, xerr.NewErrCode(xerr.DOCUMENT_UPLOAD_ERROR)
	}
	if err := mysql.Document.UpdateFilePath(opCtx, doc.ID, filePath); err != nil {
		s.markFailed(doc.ID, 0, err.Error())
		return nil, xerr.NewErrCode(xerr.DOCUMENT_UPLOAD_ERROR)
	}
	doc.FilePath = filePath

	parseStart := time.Now()
	rawText, err := s.extractTextByTika(opCtx, fileData, mimeType)
	if err != nil {
		s.markFailed(doc.ID, 0, err.Error())
		return nil, xerr.NewErrCode(xerr.DOCUMENT_UPLOAD_ERROR)
	}
	log.Printf("knowledge upload parse done doc_id=%d cost_ms=%d", doc.ID, time.Since(parseStart).Milliseconds())

	chunkStart := time.Now()
	chunks, err := s.semanticChunk(opCtx, rawText)
	if err != nil {
		s.markFailed(doc.ID, 0, err.Error())
		return nil, xerr.NewErrCode(xerr.DOCUMENT_UPLOAD_ERROR)
	}
	log.Printf("knowledge upload chunk done doc_id=%d chunks=%d cost_ms=%d", doc.ID, len(chunks), time.Since(chunkStart).Milliseconds())

	embedStart := time.Now()
	points := make([]vector.KnowledgeChunkPoint, 0, len(chunks))
	pointIDs := make([]uint64, 0, len(chunks))
	tags := cleanTags(req.Tags)
	for i, chunk := range chunks {
		vec, err := llm.GenerateEmbedding(opCtx, chunk)
		if err != nil {
			s.markFailed(doc.ID, 0, err.Error())
			return nil, xerr.NewErrCode(xerr.DOCUMENT_UPLOAD_ERROR)
		}
		pointID, err := buildKnowledgePointID(doc.ID, i)
		if err != nil {
			s.markFailed(doc.ID, 0, err.Error())
			return nil, xerr.NewErrCode(xerr.DOCUMENT_UPLOAD_ERROR)
		}
		pointIDs = append(pointIDs, pointID)
		points = append(points, vector.KnowledgeChunkPoint{
			ID:         pointID,
			Vector:     vec,
			Content:    chunk,
			KBID:       kbID,
			DocID:      doc.ID,
			DocName:    doc.FileName,
			DocType:    doc.DocType,
			Project:    doc.Project,
			Tags:       tags,
			ChunkIndex: i,
		})
	}
	log.Printf("knowledge upload embed done doc_id=%d chunks=%d cost_ms=%d", doc.ID, len(chunks), time.Since(embedStart).Milliseconds())

	upsertStart := time.Now()
	if err := vector.KnowledgeQdrant.UpsertKnowledgeChunks(opCtx, points); err != nil {
		_ = vector.KnowledgeQdrant.DeleteKnowledgePoints(opCtx, pointIDs)
		s.markFailed(doc.ID, 0, err.Error())
		return nil, xerr.NewErrCode(xerr.DOCUMENT_UPLOAD_ERROR)
	}
	log.Printf("knowledge upload qdrant upsert done doc_id=%d chunks=%d cost_ms=%d", doc.ID, len(chunks), time.Since(upsertStart).Milliseconds())

	if err := mysql.Document.UpdateStatus(opCtx, doc.ID, model.DocumentStatusReady, len(chunks), ""); err != nil {
		s.markFailed(doc.ID, len(chunks), err.Error())
		return nil, xerr.NewErrCode(xerr.DOCUMENT_UPLOAD_ERROR)
	}

	return &vo.DocumentUploadResp{
		DocID:        doc.ID,
		Status:       string(model.DocumentStatusReady),
		ChunkCount:   len(chunks),
		ProcessingMS: time.Since(start).Milliseconds(),
	}, nil
}

func (s *KnowledgeService) List(ctx context.Context, _ uint64, kbID uint64, req dto.DocumentListReq) (*vo.DocumentListResp, error) {
	if err := s.validateKnowledgeBase(kbID); err != nil {
		return nil, err
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = defaultListPageSize
	}
	if pageSize > maxListPageSize {
		pageSize = maxListPageSize
	}

	query := mysql.DocumentListQuery{
		Page:     page,
		PageSize: pageSize,
		Status:   strings.TrimSpace(req.Status),
		DocType:  strings.TrimSpace(req.DocType),
		Project:  strings.TrimSpace(req.Project),
		Tag:      strings.TrimSpace(req.Tag),
	}
	if query.Status != "" && !isValidDocumentStatus(query.Status) {
		return nil, xerr.NewErrCode(xerr.DOCUMENT_PARAM_ERROR)
	}

	docs, total, err := mysql.Document.ListByKB(ctx, kbID, query)
	if err != nil {
		return nil, xerr.NewErrCode(xerr.DOCUMENT_LIST_ERROR)
	}

	items := make([]vo.DocumentItem, 0, len(docs))
	for _, doc := range docs {
		items = append(items, vo.NewDocumentItem(doc))
	}

	return &vo.DocumentListResp{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		Items:    items,
	}, nil
}

func (s *KnowledgeService) Delete(ctx context.Context, _ uint64, kbID, docID uint64) (*vo.DocumentDeleteResp, error) {
	if err := s.validateKnowledgeBase(kbID); err != nil {
		return nil, err
	}

	timeout := time.Duration(config.AppConfig.Knowledge.UploadTimeoutSec) * time.Second
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	doc, err := mysql.Document.GetByID(opCtx, kbID, docID)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			return nil, xerr.NewErrCode(xerr.DOCUMENT_NOT_FOUND)
		}
		return nil, xerr.NewErrCode(xerr.DOCUMENT_DELETE_ERROR)
	}
	if doc.Status == model.DocumentStatusDeleted {
		return nil, xerr.NewErrCode(xerr.DOCUMENT_NOT_FOUND)
	}

	rows, err := mysql.Document.MarkDeleting(opCtx, kbID, docID)
	if err != nil {
		return nil, xerr.NewErrCode(xerr.DOCUMENT_DELETE_ERROR)
	}
	if rows == 0 {
		return nil, xerr.NewErrCode(xerr.DOCUMENT_NOT_FOUND)
	}

	deleteStart := time.Now()
	if err := vector.KnowledgeQdrant.DeleteKnowledgeByDocument(opCtx, kbID, docID); err != nil {
		s.markFailed(docID, doc.ChunkCount, err.Error())
		return nil, xerr.NewErrCode(xerr.DOCUMENT_DELETE_ERROR)
	}
	log.Printf("knowledge delete qdrant done doc_id=%d cost_ms=%d", docID, time.Since(deleteStart).Milliseconds())

	if err := deleteLocalFile(doc.FilePath); err != nil {
		s.markFailed(docID, doc.ChunkCount, err.Error())
		return nil, xerr.NewErrCode(xerr.DOCUMENT_DELETE_ERROR)
	}

	if err := mysql.Document.MarkDeleted(opCtx, docID); err != nil {
		return nil, xerr.NewErrCode(xerr.DOCUMENT_DELETE_ERROR)
	}

	return &vo.DocumentDeleteResp{
		DocID:  docID,
		Status: string(model.DocumentStatusDeleted),
	}, nil
}

func (s *KnowledgeService) validateKnowledgeBase(kbID uint64) error {
	if kbID != config.AppConfig.Knowledge.DefaultKBID {
		return xerr.NewErrCode(xerr.KNOWLEDGE_BASE_NOT_FOUND)
	}
	return nil
}

func (s *KnowledgeService) validateFileHeader(file *multipart.FileHeader) error {
	if file.Size <= 0 {
		return xerr.NewErrCode(xerr.DOCUMENT_PARAM_ERROR)
	}
	maxSizeBytes := config.AppConfig.Knowledge.UploadMaxSizeMB * 1024 * 1024
	if file.Size > maxSizeBytes {
		return xerr.NewErrCode(xerr.DOCUMENT_PARAM_ERROR)
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(file.Filename), "."))
	if !isAllowedExt(ext) {
		return xerr.NewErrCode(xerr.DOCUMENT_UNSUPPORTED_FORMAT)
	}

	mimeType := strings.TrimSpace(file.Header.Get("Content-Type"))
	if mimeType != "" && !isAllowedMimeType(mimeType) {
		return xerr.NewErrCode(xerr.DOCUMENT_UNSUPPORTED_FORMAT)
	}
	return nil
}

func readUploadFile(file *multipart.FileHeader, maxSizeBytes int64) ([]byte, error) {
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open uploaded file: %w", err)
	}
	defer src.Close()

	data, err := io.ReadAll(io.LimitReader(src, maxSizeBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read uploaded file: %w", err)
	}
	if int64(len(data)) > maxSizeBytes {
		return nil, fmt.Errorf("file exceeds max size")
	}
	return data, nil
}

func (s *KnowledgeService) storeFile(kbID, docID uint64, originName string, data []byte) (string, error) {
	baseDir := config.AppConfig.Knowledge.StorageDir
	targetDir := filepath.Join(baseDir, fmt.Sprintf("kb_%d", kbID))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create document dir: %w", err)
	}

	safeName := sanitizeFileName(originName)
	targetName := fmt.Sprintf("%d_%d_%s", docID, time.Now().UnixNano(), safeName)
	targetPath := filepath.Join(targetDir, targetName)

	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write document file: %w", err)
	}
	return targetPath, nil
}

func (s *KnowledgeService) extractTextByTika(ctx context.Context, fileData []byte, mimeType string) (string, error) {
	url := strings.TrimRight(config.AppConfig.Tika.URL, "/") + "/tika"
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(fileData))
	if err != nil {
		return "", fmt.Errorf("build tika request: %w", err)
	}
	if mimeType != "" {
		req.Header.Set("Content-Type", mimeType)
	} else {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	req.Header.Set("Accept", "text/plain")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("tika request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read tika response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("tika status=%d body=%s", resp.StatusCode, string(body))
	}

	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", fmt.Errorf("tika extracted empty content")
	}
	return text, nil
}

func (s *KnowledgeService) semanticChunk(ctx context.Context, text string) ([]string, error) {
	segments := splitInitialSegments(text)
	if len(segments) == 0 {
		return nil, fmt.Errorf("document has no valid text segments")
	}
	if len(segments) == 1 {
		return postProcessChunks(segments), nil
	}

	embeddings := make([][]float32, 0, len(segments))
	for _, segment := range segments {
		vec, err := llm.GenerateEmbedding(ctx, segment)
		if err != nil {
			return nil, fmt.Errorf("embed segment failed: %w", err)
		}
		embeddings = append(embeddings, vec)
	}

	merged := make([]string, 0, len(segments))
	current := segments[0]
	currentLen := runeLen(current)

	for i := 0; i < len(segments)-1; i++ {
		next := segments[i+1]
		sim := cosineSimilarity(embeddings[i], embeddings[i+1])
		nextLen := runeLen(next)
		if sim >= semanticSimilarityThreshold && currentLen+1+nextLen <= maxChunkChars {
			current = current + "\n" + next
			currentLen += 1 + nextLen
			continue
		}
		merged = append(merged, current)
		current = next
		currentLen = nextLen
	}
	merged = append(merged, current)

	finalChunks := postProcessChunks(merged)
	if len(finalChunks) == 0 {
		return nil, fmt.Errorf("semantic chunk produced empty result")
	}
	return finalChunks, nil
}

func splitInitialSegments(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return nil
	}

	parts := blankLineSplitRegex.Split(normalized, -1)
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if runeLen(part) <= maxChunkChars {
			segments = append(segments, part)
			continue
		}
		subParts := splitBySentences(part)
		if len(subParts) <= 1 {
			segments = append(segments, splitByRuneLength(part, maxChunkChars, splitOverlapChars)...)
			continue
		}
		for _, sub := range subParts {
			sub = strings.TrimSpace(sub)
			if sub == "" {
				continue
			}
			if runeLen(sub) <= maxChunkChars {
				segments = append(segments, sub)
			} else {
				segments = append(segments, splitByRuneLength(sub, maxChunkChars, splitOverlapChars)...)
			}
		}
	}
	return segments
}

func splitBySentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var (
		parts   []string
		builder strings.Builder
	)
	for _, r := range text {
		builder.WriteRune(r)
		if isSentenceDelimiter(r) {
			segment := strings.TrimSpace(builder.String())
			if segment != "" {
				parts = append(parts, segment)
			}
			builder.Reset()
		}
	}
	rest := strings.TrimSpace(builder.String())
	if rest != "" {
		parts = append(parts, rest)
	}
	return parts
}

func isSentenceDelimiter(r rune) bool {
	switch r {
	case '。', '！', '？', '.', '!', '?', ';', '；', '\n':
		return true
	default:
		return false
	}
}

func postProcessChunks(chunks []string) []string {
	if len(chunks) == 0 {
		return nil
	}

	mergedSmall := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		if runeLen(chunk) < minChunkChars && len(mergedSmall) > 0 {
			lastIdx := len(mergedSmall) - 1
			candidate := mergedSmall[lastIdx] + "\n" + chunk
			if runeLen(candidate) <= maxChunkChars*2 {
				mergedSmall[lastIdx] = candidate
				continue
			}
		}
		mergedSmall = append(mergedSmall, chunk)
	}

	final := make([]string, 0, len(mergedSmall))
	for _, chunk := range mergedSmall {
		if runeLen(chunk) <= maxChunkChars {
			final = append(final, chunk)
			continue
		}
		final = append(final, splitByRuneLength(chunk, maxChunkChars, splitOverlapChars)...)
	}

	return final
}

func splitByRuneLength(text string, maxLen, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= maxLen {
		return []string{text}
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= maxLen {
		overlap = maxLen / 4
	}

	step := maxLen - overlap
	if step <= 0 {
		step = maxLen
	}

	chunks := make([]string, 0, len(runes)/step+1)
	for start := 0; start < len(runes); start += step {
		end := start + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := 0; i < len(a); i++ {
		va := float64(a[i])
		vb := float64(b[i])
		dot += va * vb
		normA += va * va
		normB += vb * vb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func runeLen(text string) int {
	return len([]rune(text))
}

func cleanTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(tags))
	exists := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		if _, ok := exists[trimmed]; ok {
			continue
		}
		exists[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func sanitizeFileName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	if name == "" || name == "." {
		return "document.bin"
	}
	return name
}

func deleteLocalFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete local file: %w", err)
	}
	return nil
}

func buildKnowledgePointID(docID uint64, chunkIndex int) (uint64, error) {
	if chunkIndex < 0 {
		return 0, fmt.Errorf("invalid chunk index: %d", chunkIndex)
	}
	if chunkIndex >= (1 << chunkIDBits) {
		return 0, fmt.Errorf("chunk index too large: %d", chunkIndex)
	}
	return (docID << chunkIDBits) | uint64(chunkIndex), nil
}

func (s *KnowledgeService) markFailed(docID uint64, chunkCount int, errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = mysql.Document.UpdateStatus(ctx, docID, model.DocumentStatusFailed, chunkCount, truncateError(errMsg))
}

func truncateError(msg string) string {
	msg = strings.TrimSpace(msg)
	if len(msg) <= maxErrorMsgLen {
		return msg
	}
	return msg[:maxErrorMsgLen]
}

func errorsIsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func isValidDocumentStatus(status string) bool {
	switch model.DocumentStatus(status) {
	case model.DocumentStatusUploading,
		model.DocumentStatusProcessing,
		model.DocumentStatusReady,
		model.DocumentStatusFailed,
		model.DocumentStatusDeleting,
		model.DocumentStatusDeleted:
		return true
	default:
		return false
	}
}

func isAllowedExt(ext string) bool {
	switch strings.ToLower(ext) {
	case "pdf", "txt", "md", "markdown", "html", "htm", "csv", "doc", "docx", "ppt", "pptx", "xls", "xlsx":
		return true
	default:
		return false
	}
}

func isAllowedMimeType(mimeType string) bool {
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}
	switch strings.ToLower(mimeType) {
	case "application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/csv",
		"application/octet-stream":
		return true
	default:
		return false
	}
}
