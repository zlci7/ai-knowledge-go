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

// GenerateFromMessages 发送多轮对话请求到 LLM，返回模型的文字回复。
// 每次调用都是无状态的独立请求，携带历史消息，适合多轮对话场景。
func GenerateFromMessages(messages []Message) (string, error) {
	// 1. 构建请求体，直接使用传入的 messages
	requestBody := RequestBody{
		Model:    config.AppConfig.Dashscope.LLMModel,
		Messages: messages,
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

// GenerateNewSummary 生成新的摘要
func GenerateNewSummary(oldSummary string, oldMsgs []Message) (string, error) {
	// 格式化旧对话
	var historyStr string
	for _, m := range oldMsgs {
		historyStr += fmt.Sprintf("%s: %s\n", m.Role, m.Content)
	}

	// 2. 根据 oldSummary 是否为空，切换不同的指令
	var instruction string
	if oldSummary == "" {
		instruction = "请对以下对话内容进行总结，生成一段简洁的摘要，字数控制在 300 字以内。"
	} else {
		instruction = fmt.Sprintf(
			"已知当前的对话摘要为：\"%s\"。\n请结合以下新增的对话内容，对原有摘要进行更新和完善，确保信息连贯且完整，总字数不超过 300 字。",
			oldSummary,
		)
	}

	// 3. 构建最终 Prompt
	// 采用更结构化的模板，减少 LLM 的幻觉
	finalPrompt := fmt.Sprintf(`### 任务目标
%s

### 待处理对话内容
%s

### 摘要输出要求
1. 仅输出摘要文本，不要包含“好的”、“这是摘要”等废话。
2. 重点保留用户的问题意图和助手的核心回答。
3. 语言精炼，逻辑清晰。`, instruction, historyStr)

	// 4. 调用生成函数
	return GenerateFromSinglePrompt(finalPrompt)
}

func GenerateNewTitle(prompt string) (string, error) {
	prompt = fmt.Sprintf("请根据以下对话内容生成一个标题（不超过20个字）：\n%s", prompt)
	return GenerateFromSinglePrompt(prompt)
}
