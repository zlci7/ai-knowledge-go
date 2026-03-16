# Chat Stream 流式接口开发文档

## 1. 背景与目标

当前项目已保留原同步接口 `POST /api/chat/send`，并新增流式接口 `POST /api/chat/stream`。

流式接口目标：
- 前端在一个 HTTP 请求内持续接收模型增量输出（SSE）。
- 后端使用 `chan` 解耦 LLM 流读取与业务层消费。
- Assistant 完整回复结束后再写入 MySQL 与 Redis，避免保存半截消息。

---

## 2. 代码结构

新增或改动文件如下：

- `internal/llm/client_stream.go`
- `internal/service/chat_stream.go`
- `internal/api/handler/chat_stream.go`
- `internal/api/router/chat_router.go`
- `internal/llm/types.go`

核心职责：

- `llm` 层  
  负责调用上游模型流接口并把增量片段写入 `chan`。
- `service` 层  
  负责会话创建、上下文组装、消费 LLM `chan`、组装回复、落库、对外输出事件。
- `handler` 层  
  负责建立 SSE 响应并将 service 事件实时写给前端。

---

## 3. 接口定义

### 3.1 HTTP 接口

- 方法：`POST`
- 路径：`/api/chat/stream`
- 请求体：

```json
{
  "conversation_id": "",
  "message": "你好，帮我解释一下 RAG"
}
```

字段说明：

- `conversation_id`：可选，空字符串表示新建会话
- `message`：必填，本轮用户输入

---

## 4. SSE 事件协议

服务端响应头：

- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- `Connection: keep-alive`
- `X-Accel-Buffering: no`

事件类型：

1. `meta`  
   首包，返回会话信息
2. `delta`  
   模型增量文本
3. `done`  
   流式完成
4. `error`  
   流式中断或后处理失败

`data` 为 JSON，统一结构：

```json
{
  "type": "delta",
  "conversation_id": "1912345678901234567",
  "delta": "这是本次增量",
  "error": ""
}
```

说明：
- `delta` 仅在 `type=delta` 时有值
- `error` 仅在 `type=error` 时有值

---

## 5. 关键方法说明

### 5.1 LLM 流式方法

文件：`internal/llm/client_stream.go`

```go
func GenerateFromMessagesStream(ctx context.Context, messages []Message, out chan<- StreamChunk)
```

行为：

- 请求体包含 `stream=true`
- 逐行解析上游 `data:` 块
- 每次解析到增量文本时发送 `StreamChunk{Delta: "..."}`
- 收到结束标识后发送 `StreamChunk{Done: true}`
- 出错时发送 `StreamChunk{Err: err}`
- 方法退出时 `close(out)`

### 5.2 Service 流式方法

文件：`internal/service/chat_stream.go`

```go
func (s *ChatService) ChatStream(ctx context.Context, req dto.ChatReq) (<-chan ChatStreamEvent, error)
```

行为：

- 新会话时自动生成标题并创建会话
- 获取会话锁（当前流式锁 TTL 为 5 分钟）
- 执行 `AddAndGetContext` 组装多轮上下文
- 保存用户消息到 MySQL
- 启动 `GenerateFromMessagesStream`，消费 LLM `chan`
- 每个增量发出 `delta` 事件并拼接完整回复
- 完整回复后写入 MySQL（assistant）与 Redis
- 发出 `done` 事件

### 5.3 Handler 流式输出

文件：`internal/api/handler/chat_stream.go`

行为：

- 绑定请求参数
- 设置 SSE 响应头并 `Flush`
- 循环读取 `ChatStreamEvent` 并按 SSE 格式写回前端

---

## 6. 时序说明

1. 前端发起 `POST /api/chat/stream`
2. Handler 调用 `service.ChatStream`
3. Service 准备上下文并启动 LLM 流任务
4. LLM 每产生一段文本，写入 `llmChunks chan`
5. Service 读取 `llmChunks`，转成 `delta` 事件输出到 SSE
6. 结束后 Service 持久化 assistant 全量回复
7. Service 发送 `done`，SSE 连接结束

---

## 7. 错误与取消

- 参数错误：接口直接返回普通 JSON 错误（非 SSE）
- 流中模型错误：发送 `event:error`，随后结束
- 落库失败：发送 `event:error`，随后结束
- 前端断连：`ctx.Done()` 触发，后端停止继续发送

---

## 8. 前端联调建议

推荐使用 `fetch + ReadableStream`（因为请求是 POST 且要传 JSON body）。

核心处理点：

- 按 `\n\n` 切分 SSE 帧
- 解析 `event:` 与 `data:`
- `event=delta` 时追加到消息展示区
- `event=meta` 时记录 `conversation_id`
- `event=done` 时结束 loading
- `event=error` 时提示用户并结束流

---

## 9. 验证方式

示例（终端）：

```bash
curl -N \
  -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/chat/stream \
  -d '{"conversation_id":"","message":"给我讲讲Go中的channel"}'
```

预期看到：

- 首先收到 `event: meta`
- 然后多次收到 `event: delta`
- 最后收到 `event: done`

---

## 10. 已知限制与后续优化

当前实现：

- 已支持单模型流式回答
- 未增加心跳包（长连接超时场景可后续补 `ping` 事件）
- 锁 TTL 固定 5 分钟，超长回答场景可扩展为自动续期

后续建议：

1. 增加 `references` 事件，给 RAG 召回结果做可视化
2. 增加 `trace_id` 事件，方便日志追踪
3. 为流式链路补充集成测试（断连、超时、落库失败）
