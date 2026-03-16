package llm

import "fmt"

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

// GenerateNewTitle 生成新的标题
func GenerateNewTitle(prompt string) (string, error) {
	prompt = fmt.Sprintf("请根据以下对话内容生成一个标题（不超过20个字）：\n%s", prompt)
	return GenerateFromSinglePrompt(prompt)
}
