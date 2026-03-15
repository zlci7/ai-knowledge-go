package llm

import (
	"ai-knowledge-go/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GenerateFromSinglePrompt 发送单轮对话请求到 LLM，返回模型的文字回复。
// 每次调用都是无状态的独立请求，不携带历史消息，适合单轮问答场景。
// 如需多轮对话，应改用接收 []Message 的函数并自行维护上下文。
func GenerateFromSinglePrompt(prompt string) (string, error) {

	// 构建请求体：system 设定角色，user 携带本次输入
	requestBody := RequestBody{
		Model: config.AppConfig.Dashscope.LLMModel,
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: prompt},
		},
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// 创建 HTTP 请求，使用 OpenAI 兼容接口
	// 新加坡地域替换为：https://dashscope-intl.aliyuncs.com/...
	req, err := http.NewRequest("POST", "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.Dashscope.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	// 发送请求并等待完整响应（非流式）
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	// 解析响应体，检查错误并提取回复内容
	var result responseBody
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("llm error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}
	return result.Choices[0].Message.Content, nil
}
