package llm

// embedding请求体
type embeddingRequestBody struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingResponseBody struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// 单轮对话消息体
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestBody struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

// 多轮对话消息体
// responseBody mirrors the OpenAI-compatible response envelope.
type responseBody struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// 流式响应体
type StreamChunk struct {
	Delta string
	Done  bool
	Err   error
}

type streamResponseBody struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}
