# 长期记忆开发文档（MySQL + Redis 延迟队列 + Qdrant）

> 更新时间：2026-03-16  
> 适用代码分支：当前仓库 `ai-knowledge-go`

## 1. 目标与范围

当前长期记忆实现分为两条主链路：

1. 写入链路（用户创建/删除记忆）：
- API 先写 MySQL 并立即返回；
- 向量写入/删除通过 Redis 延迟队列异步执行；
- 失败指数退避重试，达到上限后进入死信队列（DLQ）。

2. 召回链路（流式聊天）：
- 查询重写仅用于检索；
- 用重写后的 query 做 embedding + Qdrant 检索；
- 检索结果作为长期记忆注入 system prompt。

## 2. 总体架构

```text
POST /api/memories
  -> MySQL.long_term_memories (status=pending)
  -> Redis delay_zset 入 upsert 任务
  -> 返回成功

异步 worker
  scheduler: delay_zset -> ready_list
  consumer: ready_list -> embedding(text-embedding-v3) -> Qdrant upsert/delete
  success: MySQL status=sync/deleted
  fail: 指数退避重试 -> 超限入 DLQ

POST /api/chat/stream
  -> 查询重写(仅检索，预算内)
  -> embedding + Qdrant 检索(user_id过滤, top_k=3, threshold=0.75)
  -> 注入 Prompt
  -> LLM 流式输出
```

## 3. 关键数据结构

### 3.1 MySQL: `long_term_memories`

关键字段（省略通用字段）：

- `id`: 记忆主键，同时作为 Qdrant point id
- `user_id`: 用户隔离字段
- `content`: 记忆正文
- `category`: `preference/fact/rule`
- `vector_id`: 当前实现写为 `id` 的字符串
- `vector_status`: `pending/synced/failed/deleting/deleted`
- `vector_retry_count`: 已重试次数
- `vector_last_error`: 最近一次错误
- `is_deleted`: 软删标记

模型定义见：
- `internal/model/long_term_memory.go`

### 3.2 Redis 队列键

前缀来自配置 `memory.async.queue_key_prefix`，默认 `memory:vector`：

- 延迟队列：`memory:vector:delay_zset`
- 就绪队列：`memory:vector:ready_list`
- 死信队列：`memory:vector:dlq_list`

任务体 `MemoryVectorJob`：

- `job_id`
- `op`: `upsert|delete`
- `memory_id`
- `user_id`
- `content`
- `category`
- `attempt`
- `next_run_at`
- `created_at`
- `last_error`

定义见：
- `internal/repository/redis/memory_vector_queue.go`

### 3.3 Qdrant 集合

集合名默认 `long_term_memories`，服务启动时自动确保存在。  
向量维度来自配置 `dashscope.embedding_dimension`（默认 1024），距离度量 `Cosine`。

初始化与操作见：
- `internal/repository/vector/qdrant.go`

## 4. 写入链路实现

### 4.1 创建记忆 `POST /api/memories`

流程：

1. 校验入参 `content + category`
2. 插入 MySQL，`vector_status=pending`
3. 入队 `upsert` 任务（无延迟）
4. 返回创建结果

若入队失败：标记 `failed` 并返回错误。

代码入口：

- Handler: `internal/api/handler/memory.go`
- Service: `internal/service/memory.go`
- DAO: `internal/repository/mysql/memory.go`

### 4.2 删除记忆 `DELETE /api/memories/:id`

流程：

1. MySQL 软删 `is_deleted=true`，并置 `vector_status=deleting`
2. 入队 `delete` 任务（无延迟）
3. 返回成功

若入队失败：标记 `failed` 并返回错误。

## 5. 异步消费与重试机制

服务启动时：

1. `vector.InitQdrant(...)` 初始化 Qdrant 和集合
2. `service.MemoryAsync.Start(...)` 启动异步任务：
- 调度协程：每秒将到期任务从 zset 搬到 list
- 消费协程：`BRPOP` 消费任务，执行 upsert/delete

成功回写：

- upsert 成功 -> `vector_status=synced`, `vector_id=id`, 清空错误与重试次数
- delete 成功 -> `vector_status=deleted`, 清空错误与重试次数

失败策略：

- `attempt + 1`
- 指数退避：`base * 2^(attempt-1)`（`base` 默认 2 秒）
- `attempt < retry_max`（默认 3）时重新入延迟队列
- 否则入 DLQ + MySQL 标记 `failed`

补偿机制：

- 启动时会扫描 `pending/deleting` 状态重新入队（不自动重放 `failed`）

代码：

- `internal/service/memory_async.go`

## 6. 流式召回链路实现

### 6.1 鉴权与 user 过滤

`/api/chat/send` 和 `/api/chat/stream` 已加 JWT，`user_id` 从 token 注入并透传 service。  
Qdrant 检索强制 `user_id` filter，保证用户数据隔离。

### 6.2 查询重写与检索

在 `ChatStream` 中新增前置阶段：

1. `context.WithTimeout(..., 600ms)` 创建长期记忆预算
2. 调用 `RewriteForRetrieval()` 生成检索 query（仅检索用）
3. `GenerateEmbedding()` 生成 query 向量
4. `SearchMemoryPoints(user_id, top_k=3, score_threshold=0.75)` 检索
5. 提取 `content` 列表

策略：

- 任一步失败或超时 -> 直接跳过长期记忆，不阻塞流式首包
- 不回退到“原始问题检索”

代码：

- 重写：`internal/service/retrieval_rewrite.go`
- 检索：`internal/service/memory_retrieval.go`
- Qdrant 查询：`internal/repository/vector/qdrant.go`

### 6.3 Prompt 注入顺序

当前 `ChatStream` 中实际顺序：

1. 基础 system：`You are a helpful assistant.`
2. 长期记忆 system（命中才注入，含使用约束）
3. 对话摘要 system（有摘要才注入）
4. RAG 注入位（预留，当前空）
5. 短期记忆消息窗口（Redis 返回消息，包含当前用户输入）

长期记忆 system 模板规则：

- 仅相关时使用，不相关忽略
- 不得编造未召回记忆
- 与当前输入冲突时以当前输入为准

## 7. 配置项说明

`config/config.yaml` 关键配置：

```yaml
dashscope:
  api_key: ...
  llm_model: qwen-plus
  embedding_model: text-embedding-v3
  embedding_dimension: 1024
  base_url: https://dashscope.aliyuncs.com/compatible-mode/v1

qdrant:
  host: 127.0.0.1
  port: 16333
  collection: long_term_memories

memory:
  async:
    retry_max: 3
    retry_base_seconds: 2
    queue_key_prefix: memory:vector
```

## 8. 本地运行与联调

### 8.1 启动依赖

```bash
docker compose up -d mysql redis qdrant
```

Dashboard:

- `http://<host>:16333/dashboard`

### 8.2 启动服务

启动后会自动：

1. MySQL 自动迁移（含 `long_term_memories`）
2. Redis 初始化
3. Qdrant 初始化并建集合（若不存在）
4. 启动长期记忆异步 worker

### 8.3 基本联调顺序

1. 调 `POST /api/memories` 新建记忆
2. 检查 MySQL `vector_status` 由 `pending -> synced`
3. 在 Qdrant Dashboard 查看 point
4. 调 `POST /api/chat/stream`，验证相关问题能命中长期记忆

## 9. 运维排障手册

### 9.1 Redis 队列检查

```bash
# 延迟队列
redis-cli -a 1234 -p 16379 ZRANGE memory:vector:delay_zset 0 -1

# 就绪队列
redis-cli -a 1234 -p 16379 LRANGE memory:vector:ready_list 0 -1

# 死信队列
redis-cli -a 1234 -p 16379 LRANGE memory:vector:dlq_list 0 -1
```

### 9.2 常见故障

1. `vector_status` 长时间 `pending`
- 检查 worker 是否启动
- 检查 Redis 队列是否堆积
- 检查 DashScope API Key/网络

2. 频繁进入 `failed`
- 查看 `vector_last_error`
- 检查 Qdrant 端口、集合可用性、embedding 维度一致性

3. 流式回答没用到长期记忆
- 检查是否命中 `user_id` 过滤
- 检查问题是否与记忆相关（score/threshold）
- 检查 600ms 预算是否过紧导致降级

### 9.3 死信人工干预

当前未提供管理 API，先用 `redis-cli` 处理 `dlq_list`：

1. 取出死信任务 JSON
2. 修复外部依赖（Qdrant/API key）
3. 重新入 `delay_zset`（可自定义 `next_run_at`）

## 10. 当前限制与后续建议

当前限制：

1. `failed` 状态不会在服务重启时自动重放
2. 召回结果未二次回查 MySQL `is_deleted`（异步删窗口期可出现短暂不一致）
3. RAG 注入位仅预留，尚未实现
4. 无 DLQ 管理接口，仅人工处理

建议下一步：

1. 增加 DLQ 重放管理接口（按 `memory_id` 或 `job_id`）
2. 在召回后加 MySQL 二次过滤，减少软删窗口影响
3. 增加检索日志埋点（重写耗时、召回耗时、命中分数）
4. 将 `top_k/score_threshold/timeout` 配置化
