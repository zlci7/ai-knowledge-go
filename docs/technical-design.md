# 团队智能问答知识库系统 — 技术设计文档

> **项目语言**: Go 1.25+  
> **框架**: Gin + LangChainGo  
> **版本**: v1.0 Draft  
> **日期**: 2026-03-15

---

## 一、需求概述

构建一套面向团队的智能问答知识库系统，核心能力包括：

| 能力 | 说明 |
|------|------|
| **会话短期记忆** | 滑动窗口 + 摘要压缩 + Redis 存储，保障多轮对话上下文连贯 |
| **会话长期记忆** | 主动录入 + 向量化存储 + 语义相似度召回，支撑个性化对话体验 |
| **RAG 召回** | 查询重写 + 语义分块 + 混合检索(BM25 + 向量)，提升检索精度 |

---

## 二、技术选型与可行性分析

### 2.1 核心技术栈

| 组件 | 选型 | 说明 |
|------|------|------|
| HTTP 框架 | **Gin** | Go 生态最成熟的 Web 框架，高性能，中间件丰富 |
| LLM 编排 | **LangChainGo** | Go 版 LangChain，支持 Chain/Memory/Retriever 抽象 |
| 大模型 | **通义千问 qwen-plus** (阿里百炼) | 用于答案生成、摘要压缩、查询重写 |
| Embedding 模型 | **text-embedding-v3** (阿里百炼, 1024维) | 用于文档/查询向量化 |
| 向量数据库 | **Qdrant** (推荐) | 见下方对比 |
| 关系型数据库 | **MySQL** + GORM | 用户管理、知识库元数据、对话历史持久化 |
| 缓存 | **Redis** | 短期记忆窗口、会话状态、限流 |
| 文档解析 | **Apache Tika** (HTTP 服务) + Go 原生库 | 见下方详细分析 |
| 流式输出 | **SSE (Server-Sent Events)** | Gin 原生支持，替代 Java WebFlux |

### 2.2 关于 Java 概念在 Go 中的对应

用户提到的部分组件属于 Java 生态，在 Go 中的对应方案：

| Java 组件 | Go 对应 | 说明 |
|-----------|---------|------|
| **Lombok** | **不需要** | Go 语言本身无 getter/setter 样板代码，结构体字段直接公开访问 |
| **WebFlux** (流式输出) | **Gin SSE** / `io.Writer` streaming | Gin 内置 `c.Stream()` 和 `c.SSEvent()` 支持流式推送 |
| **Spring DI** | **Wire** / 手动注入 | Go 推荐构造函数注入，复杂场景用 google/wire |

### 2.3 向量数据库对比：Qdrant vs Pinecone

| 维度 | Qdrant | Pinecone |
|------|--------|----------|
| 部署方式 | 自托管 / Qdrant Cloud | 纯托管 SaaS |
| BM25 支持 | ✅ 原生稀疏向量 (Sparse Vector) 支持 | ⚠️ 需要 Pinecone Sparse-Dense 索引 |
| 混合检索 | ✅ 单次查询同时传入 dense + sparse vector | ✅ 支持，但配置较复杂 |
| Go SDK | ✅ 官方 Go gRPC client | ✅ 官方 REST client |
| 数据隐私 | ✅ 可自托管，数据不出境 | ⚠️ 数据托管在海外 |
| 成本 | 自托管更可控 | 按用量计费 |
| 过滤能力 | ✅ 强大的 payload 过滤 | ✅ metadata 过滤 |

**推荐 Qdrant**，理由：
1. **原生支持混合检索**：dense + sparse vector 在同一个 collection 中，单次查询即可完成
2. **BM25 天然适配**：通过稀疏向量实现 BM25，无需额外搜索引擎 (如 Elasticsearch)
3. **自托管数据安全**：团队知识库可能涉及敏感信息
4. **Go gRPC 高性能**：比 REST API 延迟更低

### 2.4 文档解析方案

| 格式 | 推荐方案 | Go 库 / 服务 |
|------|---------|-------------|
| **PDF** | Apache Tika (HTTP) | `go-tika` client 调用 Tika Server |
| **Word (.docx)** | Go 原生 | `unidoc/unioffice` |
| **Markdown** | Go 原生 | `yuin/goldmark` |
| **TXT/CSV** | Go 原生 | 标准库 `os` / `encoding/csv` |
| **Excel (.xlsx)** | Go 原生 | `qax-os/excelize` |
| **HTML** | Go 原生 | `PuerkitoBio/goquery` |

**推荐组合策略**：
- **轻量格式** (MD/TXT/CSV/HTML)：Go 原生库直接解析，零外部依赖
- **复杂格式** (PDF/Word/PPT)：部署 **Apache Tika Server** (Docker 容器)，通过 HTTP 调用
- Tika 是 Apache 基金会的文档解析引擎，支持 1000+ 格式，`docker run -p 9998:9998 apache/tika` 即可启动

### 2.5 MySQL 的作用

MySQL 在本系统中承担以下职责：

| 职责 | 表 | 说明 |
|------|------|------|
| 用户管理 | `users` | 用户注册/登录/鉴权 |
| 会话管理 | `conversations` | 会话列表、标题、创建时间 |
| 消息持久化 | `messages` | 完整对话历史（Redis 短期记忆过期后的兜底） |
| 知识库管理 | `knowledge_bases` | 知识库元数据（名称、描述、所属团队） |
| 文档管理 | `documents` | 上传的文档信息（文件名、分块数、状态） |
| 长期记忆 | `long_term_memories` | 用户主动录入的记忆条目（原文 + 向量ID映射） |

---

## 三、系统架构

### 3.1 总体架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                         客户端 (Web/API)                            │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ HTTP / SSE
┌──────────────────────────────▼──────────────────────────────────────┐
│                        Gin HTTP Server                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐   │
│  │ Auth MW  │  │ Rate MW  │  │ CORS MW  │  │ Session MW       │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────────┘   │
├─────────────────────────────────────────────────────────────────────┤
│                        API Router Layer                             │
│  /api/v1/chat    /api/v1/knowledge   /api/v1/memory   /api/v1/user│
├───────────┬────────────┬──────────────┬─────────────────────────────┤
│           │            │              │                              │
│  ┌────────▼─────┐ ┌───▼──────┐ ┌────▼──────┐  ┌───────────────┐  │
│  │ Chat Service │ │ RAG      │ │ Memory    │  │ User Service  │  │
│  │              │ │ Service  │ │ Service   │  │               │  │
│  │ - 对话编排    │ │          │ │           │  │ - 注册/登录    │  │
│  │ - 流式输出    │ │ - 文档解析│ │ - 短期记忆 │  │ - JWT鉴权     │  │
│  │ - 记忆整合    │ │ - 语义分块│ │ - 长期记忆 │  │               │  │
│  └──────┬───────┘ │ - 混合检索│ │ - 记忆召回 │  └───────────────┘  │
│         │         │ - 查询重写│ │           │                      │
│         │         └───┬──────┘ └────┬──────┘                      │
├─────────┼─────────────┼─────────────┼──────────────────────────────┤
│         │    LangChainGo Abstraction Layer                         │
│  ┌──────▼─────────────▼─────────────▼──────────────────────────┐  │
│  │  LLM Provider  │  Embedder  │  VectorStore  │  Memory      │  │
│  │  (qwen-plus)   │  (v3-1024) │  (Qdrant)     │  (Redis)     │  │
│  └──────┬──────────┴──────┬─────┴──────┬────────┴──────┬───────┘  │
├─────────┼─────────────────┼────────────┼───────────────┼──────────┤
│         │    Infrastructure Layer                      │          │
│  ┌──────▼──────┐  ┌──────▼──────┐  ┌──▼────┐  ┌──────▼──────┐  │
│  │ 阿里百炼 API │  │   Qdrant    │  │ MySQL │  │    Redis    │  │
│  │ (LLM+Embed) │  │ (向量存储)   │  │(元数据)│  │ (短期记忆)   │  │
│  └─────────────┘  └─────────────┘  └───────┘  └─────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.2 项目目录结构

```
ai-knowledge-go/
├── cmd/
│   └── server/
│       └── main.go                 # 入口
├── internal/
│   ├── config/
│   │   └── config.go               # 配置加载 (Viper)
│   ├── middleware/
│   │   ├── auth.go                 # JWT 认证
│   │   ├── ratelimit.go            # 限流
│   │   └── cors.go                 # CORS
│   ├── router/
│   │   └── router.go               # 路由注册
│   ├── handler/                     # HTTP Handler (Controller)
│   │   ├── chat.go
│   │   ├── knowledge.go
│   │   ├── memory.go
│   │   └── user.go
│   ├── service/                     # 业务逻辑
│   │   ├── chat.go                 # 对话编排
│   │   ├── rag.go                  # RAG 检索
│   │   ├── memory.go               # 记忆管理
│   │   ├── knowledge.go            # 知识库管理
│   │   ├── document.go             # 文档解析与分块
│   │   └── user.go                 # 用户管理
│   ├── model/                       # 数据模型
│   │   ├── user.go
│   │   ├── conversation.go
│   │   ├── message.go
│   │   ├── knowledge.go
│   │   └── memory.go
│   ├── repository/                  # 数据访问层
│   │   ├── mysql/
│   │   │   ├── user.go
│   │   │   ├── conversation.go
│   │   │   └── knowledge.go
│   │   ├── redis/
│   │   │   └── memory.go
│   │   └── vector/
│   │       └── qdrant.go
│   ├── llm/                         # LLM 相关封装
│   │   ├── dashscope.go            # 阿里百炼 LLM Client
│   │   ├── embedder.go             # Embedding 封装
│   │   └── rewriter.go             # 查询重写
│   ├── rag/                         # RAG 核心逻辑
│   │   ├── chunker.go              # 语义分块
│   │   ├── retriever.go            # 混合检索器
│   │   └── bm25.go                 # BM25 稀疏向量生成
│   └── pkg/                         # 通用工具
│       ├── response.go
│       ├── errors.go
│       └── jwt.go
├── config/
│   └── config.yaml                  # 配置文件
├── docs/
│   └── technical-design.md          # 本文档
├── go.mod
├── go.sum
└── Makefile
```

---

## 四、核心模块详细设计

### 4.1 会话短期记忆：滑动窗口 + 摘要 + Redis

#### 4.1.1 设计目标

- 保持最近 N 轮对话的完整上下文
- 当窗口滑动时，旧消息被 LLM 摘要压缩，不丢失关键信息
- 所有状态存储在 Redis 中，支持分布式部署

#### 4.1.2 数据结构

```
Redis Key 设计：
  
  # 当前会话的消息滑动窗口 (List, 最新的在右端)
  stm:{conversation_id}:messages    →  List<JSON(Message)>
  
  # 窗口滑动时的累积摘要
  stm:{conversation_id}:summary     →  String (摘要文本)
  
  # 会话元数据
  stm:{conversation_id}:meta        →  Hash { user_id, created_at, last_active }

  TTL: 所有 key 设置 24h 过期 (可配置)，活跃会话自动续期
```

#### 4.1.3 核心流程

```
用户发送消息
     │
     ▼
┌─────────────────────────┐
│ 1. 从 Redis 读取当前窗口 │
│    messages + summary    │
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│ 2. 检查窗口大小          │
│    len(messages) >= N ?  │──── 否 ───▶ 直接追加新消息
│    (N=10, 可配置)        │
└────────────┬────────────┘
             │ 是
             ▼
┌─────────────────────────────────────────┐
│ 3. 取出窗口最旧的 K 条消息 (K=4)         │
│    调用 LLM 生成摘要:                     │
│    prompt = "请将以下对话摘要为简洁的要点:  │
│              旧摘要: {old_summary}         │
│              新对话: {oldest_k_messages}"  │
│    new_summary = LLM(prompt)              │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────┐
│ 4. 更新 Redis:                      │
│    - LTRIM 移除最旧 K 条             │
│    - SET 新 summary                  │
│    - RPUSH 新用户消息                 │
│    - EXPIRE 续期 24h                 │
└────────────┬────────────────────────┘
             │
             ▼
┌─────────────────────────────────────┐
│ 5. 构建 LLM 上下文:                  │
│    System: "你是团队知识助手..."       │
│    + "[对话摘要]: {summary}"         │
│    + 窗口内全部 messages              │
│    + 当前用户消息                     │
└─────────────────────────────────────┘
```

#### 4.1.4 关键参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `window_size` | 10 | 滑动窗口保留的消息对数 (user+assistant 算 2 条) |
| `slide_step` | 4 | 每次滑动移除的消息数 |
| `summary_max_tokens` | 300 | 摘要最大 token 数 |
| `ttl` | 24h | 非活跃会话过期时间 |

#### 4.1.5 Go 伪代码

```go
type ShortTermMemory struct {
    rdb       *redis.Client
    llm       llms.Model
    windowSize int
    slideStep  int
}

func (m *ShortTermMemory) AddAndGetContext(ctx context.Context, convID string, userMsg Message) ([]Message, string, error) {
    key := fmt.Sprintf("stm:%s:messages", convID)
    summaryKey := fmt.Sprintf("stm:%s:summary", convID)
    
    // 读取当前窗口
    raw, _ := m.rdb.LRange(ctx, key, 0, -1).Result()
    messages := deserializeMessages(raw)
    summary, _ := m.rdb.Get(ctx, summaryKey).Result()
    
    // 窗口满了 → 滑动 + 摘要
    if len(messages) >= m.windowSize {
        oldest := messages[:m.slideStep]
        summary = m.summarize(ctx, summary, oldest)
        
        // 原子操作: 移除旧消息 + 更新摘要
        pipe := m.rdb.Pipeline()
        pipe.LTrim(ctx, key, int64(m.slideStep), -1)
        pipe.Set(ctx, summaryKey, summary, 24*time.Hour)
        pipe.Exec(ctx)
        
        messages = messages[m.slideStep:]
    }
    
    // 追加新消息
    m.rdb.RPush(ctx, key, serialize(userMsg))
    m.rdb.Expire(ctx, key, 24*time.Hour)
    
    return messages, summary, nil
}

func (m *ShortTermMemory) summarize(ctx context.Context, oldSummary string, msgs []Message) string {
    prompt := fmt.Sprintf(
        "请将以下对话内容与已有摘要合并，生成简洁的摘要要点（不超过300字）：\n\n"+
        "已有摘要：%s\n\n新对话：\n%s",
        oldSummary, formatMessages(msgs),
    )
    result, _ := llms.GenerateFromSinglePrompt(ctx, m.llm, prompt)
    return result
}
```

#### 4.1.6 持久化兜底

Redis 中的短期记忆有 TTL，过期后消息丢失。为此在每次对话结束（或异步定时）将完整消息写入 MySQL `messages` 表，实现持久化兜底。用户可通过「历史对话」功能查看。

---

### 4.2 会话长期记忆：主动录入 + 向量化 + 语义召回

#### 4.2.1 设计目标

- 用户/管理员可主动录入重要信息作为「长期记忆」
- 记忆内容向量化后存入 Qdrant，支持语义相似度召回
- 对话时自动检索相关记忆，注入 LLM 上下文

#### 4.2.2 长期记忆 vs 知识库文档

| 维度 | 长期记忆 | 知识库文档 |
|------|---------|-----------|
| 录入方式 | 用户主动输入 / 对话中提取 | 管理员上传文档 |
| 粒度 | 单条事实/偏好（短文本） | 完整文档 → 分块 |
| 示例 | "张三偏好用 Python"、"项目X的截止日期是6月30日" | 技术规范.pdf、API文档.md |
| 存储 | MySQL (原文) + Qdrant (向量) | MySQL (元数据) + Qdrant (分块向量) |
| Qdrant Collection | `long_term_memories` | `knowledge_chunks` |
| 召回策略 | 纯向量语义检索 (top-3) | 混合检索 BM25 + 向量 (top-5) |

#### 4.2.3 数据模型

```sql
CREATE TABLE long_term_memories (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id     BIGINT NOT NULL,
    content     TEXT NOT NULL,          -- 记忆原文
    category    VARCHAR(50),            -- 分类: preference/fact/rule
    vector_id   VARCHAR(100),           -- Qdrant 中的 point ID
    source      VARCHAR(20) DEFAULT 'manual',  -- manual / extracted
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    is_deleted  TINYINT DEFAULT 0,
    INDEX idx_user (user_id),
    INDEX idx_category (user_id, category)
);
```

Qdrant collection `long_term_memories`:
```json
{
  "vectors": {
    "size": 1024,
    "distance": "Cosine"
  },
  "payload_schema": {
    "user_id": "integer",
    "category": "keyword",
    "content": "text",
    "created_at": "datetime"
  }
}
```

#### 4.2.4 录入流程

```
用户输入: "记住: 我偏好使用 Vim 编辑器"
     │
     ▼
┌────────────────────────────────────┐
│ 1. 解析意图 (是否为记忆录入指令)    │
│    - 关键词匹配: "记住"/"记录"      │
│    - 或 LLM 意图分类                │
└──────────────┬─────────────────────┘
               │
               ▼
┌────────────────────────────────────┐
│ 2. 提取记忆内容                     │
│    content = "偏好使用 Vim 编辑器"   │
│    category = "preference"          │
└──────────────┬─────────────────────┘
               │
               ▼
┌────────────────────────────────────┐
│ 3. 调用 Embedding API              │
│    vector = embed(content)   // 1024维│
└──────────────┬─────────────────────┘
               │
               ▼
┌────────────────────────────────────┐
│ 4. 写入 Qdrant + MySQL             │
│    Qdrant: upsert point            │
│    MySQL:  INSERT long_term_memories│
└────────────────────────────────────┘
```

#### 4.2.5 召回流程

```
用户发送普通问题
     │
     ▼
┌────────────────────────────────────┐
│ 1. 将用户问题向量化                  │
│    query_vec = embed(user_query)    │
└──────────────┬─────────────────────┘
               │
               ▼
┌────────────────────────────────────┐
│ 2. 在 Qdrant long_term_memories    │
│    中检索 (过滤 user_id)            │
│    top_k = 3, score_threshold=0.75 │
└──────────────┬─────────────────────┘
               │
               ▼
┌────────────────────────────────────┐
│ 3. 将召回的记忆注入 System Prompt   │
│    "[用户记忆]:                      │
│     - 偏好使用 Vim 编辑器            │
│     - 正在学习 Rust 语言"            │
└────────────────────────────────────┘
```

---

### 4.3 RAG 检索增强生成

#### 4.3.1 文档处理 Pipeline

> **前置假设**：已实现 Web 后台用于文档上传，上传时用户需填写元数据/标签（文档类型、所属项目等），
> 这些元数据会贯穿整个 Pipeline，最终写入 Qdrant 的 payload 中，用于检索时的精准过滤。

```
┌─ Step 1: 上传文件 ──────────────────────────────────────────────┐
│  用户通过 Web 后台上传文档                                        │
│  同时填写元数据:                                                  │
│    - doc_type: "项目资料" / "研发规范" / "会议纪要" / ...         │
│    - project:  "项目A" / "项目B" / ...                           │
│    - tags:     ["部署", "架构"] (自定义标签)                      │
│  → 文件存储到服务器/OSS，元数据写入 MySQL documents 表            │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               ▼
┌─ Step 2: 文档解析 (Tika) ───────────────────────────────────────┐
│  调用 Apache Tika Server 将文档提取为纯文本                       │
│  tikaClient.Parse(file) → rawText                                │
│                                                                   │
│  ► 为什么用 Tika？                                                │
│    统一处理 PDF/Word/PPT/HTML 等异构格式，一个 HTTP 调用搞定      │
│    避免为每种格式引入不同的 Go 库，降低维护成本                     │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               ▼
┌─ Step 3: 构建 Document + 注入元数据 ───────────────────────────┐
│  将 rawText 包装为 Document 对象，附加用户填写的元数据:           │
│                                                                  │
│  Document {                                                      │
│    Content:  rawText,                                            │
│    Metadata: {                                                   │
│      "doc_id":     123,                                          │
│      "doc_name":   "K8s部署手册.pdf",                            │
│      "doc_type":   "研发规范",                                    │
│      "project":    "项目A",                                      │
│      "tags":       ["部署", "K8s"],                              │
│      "kb_id":      1,        // 所属知识库                       │
│      "uploaded_by": 42,      // 上传者                           │
│    }                                                             │
│  }                                                               │
│                                                                  │
│  ► 为什么要在这一步注入元数据？                                    │
│    元数据会随分块一起写入 Qdrant payload，检索时可按                │
│    doc_type/project 做过滤，大幅缩小搜索范围，提升召回精度         │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               ▼
┌─ Step 4: 语义分块 → []TextSegment ──────────────────────────────┐
│  基于余弦相似度的语义分块（详见 4.3.2）                           │
│  SemanticChunker.Chunk(document) → []TextSegment                 │
│                                                                   │
│  每个 TextSegment 继承 Document 的全部 Metadata                   │
│  并追加 chunk 级别的元数据:                                       │
│    chunk_index: 0, 1, 2, ...                                     │
│    section:     "第三章 部署架构" (如果能从文档结构中提取)         │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               ▼
┌─ Step 5: 向量化 ────────────────────────────────────────────────┐
│  调用 text-embedding-v3 (1024维) 对每个 TextSegment 生成:        │
│    - dense vector: embed(segment.Content)     // 语义向量        │
│    - sparse vector: bm25Encode(segment.Content) // BM25 稀疏向量 │
│                                                                   │
│  ► 为什么同时生成两种向量？                                       │
│    dense 负责语义匹配 ("部署架构" ↔ "上线流程")                   │
│    sparse 负责关键词精确匹配 ("CrashLoopBackOff" 必须完全命中)    │
│    两者互补，混合检索效果远优于单一检索                             │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               ▼
┌─ Step 6: 存入 Qdrant ──────────────────────────────────────────┐
│  Qdrant.Upsert(collection="knowledge_chunks", points=[          │
│    {                                                             │
│      id:      uuid,                                              │
│      vectors: { "dense": [0.12, ...], "bm25": {sparse} },       │
│      payload: {                                                  │
│        content:    "原始文本内容...",                              │
│        doc_id:     123,                                          │
│        doc_name:   "K8s部署手册.pdf",                            │
│        doc_type:   "研发规范",         ← 元数据                  │
│        project:    "项目A",            ← 元数据                  │
│        tags:       ["部署", "K8s"],    ← 元数据                  │
│        kb_id:      1,                                            │
│        chunk_index: 0,                                           │
│        section:    "第三章 部署架构",                              │
│      }                                                           │
│    },                                                            │
│    ...                                                           │
│  ])                                                              │
│                                                                   │
│  同时更新 MySQL documents 表:                                     │
│    status = 'ready', chunk_count = len(chunks)                   │
└─────────────────────────────────────────────────────────────────┘
```

> **小结**：整个 Pipeline 的代码实现本质上是调用各服务的 API（Tika 解析、Embedding API、Qdrant upsert），
> 实现难度不高。**设计上的重点**在于：元数据从上传→分块→存储的全链路贯穿，以及语义分块策略的选择。

#### 4.3.2 语义分块策略（核心创新点）

**问题**：固定大小分块 (如每 512 token) 会在语义中间截断，导致上下文丢失。

**解决方案**：基于余弦相似度的语义分块。

**算法流程**：

```
Step 1: 将文档按句子分割
        sentences = split_by_sentence(document)

Step 2: 为每个句子生成 embedding
        embeddings = [embed(s) for s in sentences]

Step 3: 计算相邻句子的余弦相似度
        similarities[i] = cosine(embeddings[i], embeddings[i+1])

Step 4: 找出相似度低谷 (语义断裂点)
        breakpoints = find_valleys(similarities, threshold=0.3)
        
        相似度曲线示意:
        1.0 ┤
            │   ╭─╮     ╭──╮    ╭─╮
        0.7 ┤──╯   ╰───╯    ╰──╯   ╰──
            │         ▲          ▲
        0.3 ┤─ ─ ─ ─ ┼─ ─ ─ ─ ┼─ ─ ─ ─  threshold
        0.0 ┤         │          │
            └──1──2──3──4──5──6──7──8──▶ 句子编号
                      ↑          ↑
                  断裂点1      断裂点2
                  
        → Chunk1: [句1, 句2, 句3]
        → Chunk2: [句4, 句5, 句6]  
        → Chunk3: [句7, 句8]

Step 5: 后处理
        - 过小的 chunk (< 100字) 合并到相邻 chunk
        - 过大的 chunk (> 1000字) 再次分割
        - 每个 chunk 添加上下文前缀: "文档:{doc_name}, 章节:{section}"
```

**Go 伪代码**：

```go
type SemanticChunker struct {
    embedder      embeddings.Embedder
    threshold     float64  // 断裂点阈值, 默认 0.3
    minChunkSize  int      // 最小 chunk 字数
    maxChunkSize  int      // 最大 chunk 字数
}

func (c *SemanticChunker) Chunk(ctx context.Context, text string) ([]Chunk, error) {
    sentences := splitBySentence(text)
    
    // 批量向量化
    vecs, err := c.embedder.EmbedDocuments(ctx, sentences)
    if err != nil {
        return nil, err
    }
    
    // 计算相邻句子相似度
    similarities := make([]float64, len(vecs)-1)
    for i := 0; i < len(vecs)-1; i++ {
        similarities[i] = cosineSimilarity(vecs[i], vecs[i+1])
    }
    
    // 找断裂点 (相似度低于阈值)
    breakpoints := []int{}
    for i, sim := range similarities {
        if sim < c.threshold {
            breakpoints = append(breakpoints, i+1)
        }
    }
    
    // 按断裂点分块
    chunks := splitByBreakpoints(sentences, breakpoints)
    
    // 后处理: 合并过小、分割过大
    return c.postProcess(chunks), nil
}
```

#### 4.3.3 BM25 稀疏向量生成

Qdrant 支持 Sparse Vector，我们利用这一特性实现 BM25 检索，无需额外部署 Elasticsearch。

**方案**：在文档入库时，同时生成 dense vector (语义) + sparse vector (BM25 权重)。

```go
type BM25Encoder struct {
    idf       map[string]float64  // 逆文档频率
    avgDocLen float64             // 平均文档长度
    k1        float64             // 1.2
    b         float64             // 0.75
}

// 对单个文档生成稀疏向量
func (e *BM25Encoder) Encode(text string) SparseVector {
    tokens := tokenize(text) // jieba 分词
    tf := countTermFreq(tokens)
    docLen := float64(len(tokens))
    
    indices := []uint32{}
    values := []float32{}
    
    for term, freq := range tf {
        idf := e.idf[term]
        tfNorm := (freq * (e.k1 + 1)) / (freq + e.k1*(1-e.b+e.b*docLen/e.avgDocLen))
        score := idf * tfNorm
        
        indices = append(indices, hashTerm(term))
        values = append(values, float32(score))
    }
    
    return SparseVector{Indices: indices, Values: values}
}
```

**注意**：Go 生态中分词可使用 `go-ego/gse` (Go 中文分词库，类似 jieba)。

#### 4.3.4 查询重写策略

用户原始查询可能口语化、模糊、带指代，通过 LLM 重写提升检索精度。

```
用户原始查询: "上次说的那个部署的问题怎么解决的"
     │
     ▼
┌──────────────────────────────────────────────┐
│ Query Rewriter (LLM)                         │
│                                              │
│ System: 你是查询重写助手。基于对话上下文，     │
│ 将用户的模糊查询重写为清晰、具体的检索查询。   │
│ 输出 2-3 个不同角度的查询。                    │
│                                              │
│ 对话上下文: [最近5条消息]                      │
│ 用户查询: "上次说的那个部署的问题怎么解决的"    │
│                                              │
│ 输出:                                         │
│ 1. "Kubernetes 部署 Pod CrashLoopBackOff 解决方案" │
│ 2. "K8s 部署失败 内存溢出 排查步骤"              │
│ 3. "容器部署 OOM 问题修复"                       │
└──────────────────────────────────────────────┘
```

**多查询策略**：生成 2-3 个重写查询，分别检索后取并集 + 去重 + 重排序。

#### 4.3.5 混合检索流程

```
重写后的查询 (多条)
     │
     ▼
┌───────────────────────────────────────────────────────────┐
│                    对每条查询并行执行:                      │
│                                                           │
│  ┌─────────────────┐        ┌─────────────────────────┐  │
│  │  Dense Vector    │        │  Sparse Vector (BM25)   │  │
│  │  检索            │        │  检索                    │  │
│  │                  │        │                          │  │
│  │  query_vec =     │        │  query_sparse =          │  │
│  │  embed(query)    │        │  bm25_encode(query)      │  │
│  │                  │        │                          │  │
│  │  Qdrant search   │        │  Qdrant search           │  │
│  │  (dense, top=10) │        │  (sparse, top=10)        │  │
│  └────────┬────────┘        └────────────┬─────────────┘  │
│           │                              │                │
│           └──────────┬───────────────────┘                │
│                      ▼                                    │
│           ┌──────────────────────┐                        │
│           │  Reciprocal Rank     │                        │
│           │  Fusion (RRF)        │                        │
│           │                      │                        │
│           │  score = Σ 1/(k+rank)│                        │
│           │  k = 60 (常数)       │                        │
│           └──────────┬───────────┘                        │
│                      │                                    │
└──────────────────────┼────────────────────────────────────┘
                       ▼
              ┌──────────────────┐
              │  去重 + 取 Top-5  │
              │  作为最终上下文    │
              └──────────────────┘
```

**RRF (Reciprocal Rank Fusion)** 融合公式：

```
RRF_score(doc) = Σ  1 / (k + rank_i(doc))
                 i∈{dense, sparse, query1, query2, ...}

k = 60 (经验常数，平衡高排名和低排名的影响)
```

#### 4.3.6 Qdrant Collection 设计

```json
{
  "collection_name": "knowledge_chunks",
  "vectors": {
    "dense": {
      "size": 1024,
      "distance": "Cosine"
    }
  },
  "sparse_vectors": {
    "bm25": {}
  },
  "payload_schema": {
    "kb_id": "integer",
    "doc_id": "integer",
    "doc_name": "keyword",
    "doc_type": "keyword",
    "project": "keyword",
    "tags": "keyword[]",
    "chunk_index": "integer",
    "section": "keyword",
    "content": "text",
    "created_at": "datetime"
  }
}
```

> **元数据过滤的价值**：用户问"项目A 的部署架构"时，除了语义检索，还可以加上 
> `filter: { project = "项目A" }` 精准缩小范围，避免跨项目的无关内容干扰。
```

检索时使用 Qdrant 的 `query_points` API，同时传入 dense 和 sparse vector，由 Qdrant 内部进行 RRF 融合 (Qdrant 1.7+ 支持 `prefetch` + `fusion` 参数)。

---

### 4.4 完整对话流程（端到端）

```
用户发送消息: "我们项目的部署架构是怎样的？"
     │
     ▼
┌─ Step 1: 召回短期记忆 ─────────────────────────────────────┐
│  从 Redis 读取: messages 窗口 + summary                      │
│  如窗口已满 → 滑动 + 摘要压缩                                │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌─ Step 2: 查询重写 (上下文聚合 + 改写) ─────────────────────┐
│  将短期记忆上下文 + 用户原始问题交给 LLM 重写                 │
│  输入: summary + recent_messages + 用户问题                   │
│  输出: 2~3 条清晰、去指代、多角度的检索查询                    │
│  → ["K8s ArgoCD 部署架构设计", "微服务部署流程 CI/CD"]        │
│                                                               │
│  ► 为什么先重写再检索？                                       │
│    用户原始问题可能含指代词("上次那个")、口语化、信息缺失       │
│    重写后的查询更精准，后续长期记忆召回 + RAG 检索质量更高      │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌─ Step 3: 召回长期记忆 ─────────────────────────────────────┐
│  将重写后的查询向量化                                         │
│  query_vec = embed(rewritten_queries[0])                      │
│  Qdrant.search(collection="long_term_memories",               │
│                filter={user_id}, top_k=3, threshold=0.75)     │
│  → 召回: ["项目使用 K8s + ArgoCD 部署", ...]                  │
│                                                               │
│  ► 为什么用重写后的查询而非原始问题？                          │
│    重写后的查询已消除指代、补充上下文，语义更完整               │
│    向量化后在长期记忆中的匹配准确率更高                         │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌─ Step 4: RAG 混合检索 ─────────────────────────────────────┐
│  对每条重写查询，在 knowledge_chunks 中执行:                  │
│    - Dense Vector 检索 (语义匹配)                            │
│    - Sparse Vector 检索 (BM25 关键词匹配)                    │
│  Qdrant 服务端自动做 RRF 融合 → 取 Top-5 chunks             │
│                                                               │
│  ► 混合检索的分工:                                            │
│    我方负责: 生成 dense vector + sparse vector (BM25编码)     │
│    Qdrant 负责: prefetch 两路结果 + RRF 融合排序              │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌─ Step 5: Prompt 组装 ──────────────────────────────────────┐
│  System Prompt:                                              │
│    "你是团队知识助手，基于以下信息回答问题。                    │
│     如果信息不足，请诚实说明。"                                │
│                                                              │
│  [对话摘要]: {summary}                                       │
│  [用户记忆]: {long_term_memories}                            │
│  [知识库参考]:                                                │
│    [1] {chunk_1_content} —— 来源: {doc_name}                 │
│    [2] {chunk_2_content} —— 来源: {doc_name}                 │
│    ...                                                       │
│  [历史对话]: {recent_messages}                                │
│  [用户问题]: "我们项目的部署架构是怎样的？"                    │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌─ Step 6: 调用 LLM + 流式输出 ─────────────────────────────┐
│  调用 qwen-plus API (stream=true)                            │
│  通过 Gin SSE 逐 token 推送给前端                             │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌─ Step 7: 更新短期记忆 ────────────────────────────────────┐
│  - 将本轮 user 消息 + assistant 完整回复写入 Redis 窗口      │
│  - 异步写入 MySQL messages 表 (持久化兜底)                   │
└─────────────────────────────────────────────────────────────┘
```

---

## 五、API 接口设计

### 5.1 对话相关

| Method | Path | 说明 |
|--------|------|------|
| POST | `/api/v1/chat/completions` | 发送消息 (SSE 流式响应) |
| GET | `/api/v1/conversations` | 获取会话列表 |
| GET | `/api/v1/conversations/:id/messages` | 获取历史消息 |
| POST | `/api/v1/conversations` | 创建新会话 |
| DELETE | `/api/v1/conversations/:id` | 删除会话 |

#### 请求示例: 发送消息

```json
POST /api/v1/chat/completions
Content-Type: application/json
Authorization: Bearer <jwt_token>

{
  "conversation_id": "conv_abc123",
  "message": "我们项目的部署架构是怎样的？",
  "knowledge_base_ids": [1, 2],
  "stream": true
}
```

#### SSE 响应格式

```
event: message
data: {"content": "根据", "role": "assistant"}

event: message
data: {"content": "知识库", "role": "assistant"}

...

event: message
data: {"content": "", "role": "assistant", "finish_reason": "stop", "references": [{"doc_name": "部署手册.md", "chunk": "..."}]}
```

### 5.2 知识库相关

| Method | Path | 说明 |
|--------|------|------|
| POST | `/api/v1/knowledge-bases` | 创建知识库 |
| GET | `/api/v1/knowledge-bases` | 知识库列表 |
| POST | `/api/v1/knowledge-bases/:id/documents` | 上传文档 |
| GET | `/api/v1/knowledge-bases/:id/documents` | 文档列表 |
| DELETE | `/api/v1/knowledge-bases/:id/documents/:doc_id` | 删除文档 |
| POST | `/api/v1/knowledge-bases/:id/reindex` | 重新索引 |

### 5.3 长期记忆

| Method | Path | 说明 |
|--------|------|------|
| POST | `/api/v1/memories` | 新增记忆 |
| GET | `/api/v1/memories` | 记忆列表 |
| PUT | `/api/v1/memories/:id` | 修改记忆 |
| DELETE | `/api/v1/memories/:id` | 删除记忆 |

### 5.4 用户相关

| Method | Path | 说明 |
|--------|------|------|
| POST | `/api/v1/auth/register` | 注册 |
| POST | `/api/v1/auth/login` | 登录 (返回 JWT) |
| GET | `/api/v1/users/me` | 当前用户信息 |

---

## 六、数据库设计

### 6.1 MySQL ER 概览

```
users (用户)
  ├── conversations (会话)  [1:N]
  │     └── messages (消息)  [1:N]
  ├── long_term_memories (长期记忆)  [1:N]
  └── user_knowledge_bases (用户-知识库关联)  [M:N]
           └── knowledge_bases (知识库)
                 └── documents (文档)  [1:N]
```

### 6.2 核心建表 SQL

```sql
-- 用户表
CREATE TABLE users (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    username    VARCHAR(50) UNIQUE NOT NULL,
    email       VARCHAR(100) UNIQUE NOT NULL,
    password    VARCHAR(255) NOT NULL,  -- bcrypt hash
    avatar_url  VARCHAR(500),
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 会话表
CREATE TABLE conversations (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id     BIGINT NOT NULL,
    title       VARCHAR(200),
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    is_deleted  TINYINT DEFAULT 0,
    FOREIGN KEY (user_id) REFERENCES users(id),
    INDEX idx_user (user_id)
);

-- 消息表 (短期记忆过期后的持久化存储)
CREATE TABLE messages (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    conversation_id BIGINT NOT NULL,
    role            ENUM('user', 'assistant', 'system') NOT NULL,
    content         TEXT NOT NULL,
    token_count     INT,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id),
    INDEX idx_conv (conversation_id)
);

-- 知识库表
CREATE TABLE knowledge_bases (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    created_by  BIGINT NOT NULL,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- 用户-知识库关联 (多对多)
CREATE TABLE user_knowledge_bases (
    user_id         BIGINT NOT NULL,
    knowledge_base_id BIGINT NOT NULL,
    role            ENUM('owner', 'editor', 'viewer') DEFAULT 'viewer',
    PRIMARY KEY (user_id, knowledge_base_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id)
);

-- 文档表 (含用户上传时填写的元数据/标签)
CREATE TABLE documents (
    id                BIGINT PRIMARY KEY AUTO_INCREMENT,
    knowledge_base_id BIGINT NOT NULL,
    file_name         VARCHAR(255) NOT NULL,
    file_type         VARCHAR(20),         -- pdf, md, docx, txt
    file_size         BIGINT,
    doc_type          VARCHAR(50),         -- 文档类型: 项目资料 / 研发规范 / 会议纪要 / API文档
    project           VARCHAR(100),        -- 所属项目
    tags              JSON,                -- 自定义标签, e.g. ["部署", "K8s", "架构"]
    chunk_count       INT DEFAULT 0,       -- 分块数量
    status            ENUM('uploading', 'processing', 'ready', 'failed') DEFAULT 'uploading',
    error_message     TEXT,
    uploaded_by       BIGINT NOT NULL,
    created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id),
    FOREIGN KEY (uploaded_by) REFERENCES users(id),
    INDEX idx_kb (knowledge_base_id),
    INDEX idx_doc_type (doc_type),
    INDEX idx_project (project)
);

-- 长期记忆表
CREATE TABLE long_term_memories (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id     BIGINT NOT NULL,
    content     TEXT NOT NULL,
    category    VARCHAR(50),             -- preference / fact / rule
    vector_id   VARCHAR(100),            -- Qdrant point UUID
    source      VARCHAR(20) DEFAULT 'manual',
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    is_deleted  TINYINT DEFAULT 0,
    FOREIGN KEY (user_id) REFERENCES users(id),
    INDEX idx_user (user_id),
    INDEX idx_category (user_id, category)
);
```

---

## 七、关键依赖库

| 库 | 用途 | Go Module |
|----|------|-----------|
| Gin | HTTP 框架 | `github.com/gin-gonic/gin` |
| LangChainGo | LLM 编排框架 | `github.com/tmc/langchaingo` |
| GORM | MySQL ORM | `gorm.io/gorm` + `gorm.io/driver/mysql` |
| go-redis | Redis 客户端 | `github.com/redis/go-redis/v9` |
| Qdrant Go | 向量数据库客户端 | `github.com/qdrant/go-client` |
| Viper | 配置管理 | `github.com/spf13/viper` |
| jwt-go | JWT 鉴权 | `github.com/golang-jwt/jwt/v5` |
| gse | 中文分词 (BM25) | `github.com/go-ego/gse` |
| goldmark | Markdown 解析 | `github.com/yuin/goldmark` |
| excelize | Excel 解析 | `github.com/xuri/excelize/v2` |
| go-tika | Tika 客户端 (PDF/Word) | `github.com/google/go-tika` |
| uuid | ID 生成 | `github.com/google/uuid` |
| zap | 结构化日志 | `go.uber.org/zap` |

---

## 八、部署架构

### 8.1 Docker Compose (开发环境)

```yaml
version: "3.8"
services:
  app:
    build: .
    ports: ["8080:8080"]
    depends_on: [mysql, redis, qdrant, tika]
    environment:
      - CONFIG_PATH=/app/config/config.yaml
    volumes:
      - ./config:/app/config

  mysql:
    image: mysql:8.0
    ports: ["3306:3306"]
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: ai_knowledge
    volumes:
      - mysql_data:/var/lib/mysql

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"   # REST API
      - "6334:6334"   # gRPC
    volumes:
      - qdrant_data:/qdrant/storage

  tika:
    image: apache/tika:latest
    ports: ["9998:9998"]

volumes:
  mysql_data:
  qdrant_data:
```

### 8.2 配置文件示例

```yaml
# config/config.yaml
server:
  port: 8080
  mode: debug  # debug / release

mysql:
  host: localhost
  port: 3306
  database: ai_knowledge
  username: root
  password: root
  max_open_conns: 20

redis:
  addr: localhost:6379
  password: ""
  db: 0

qdrant:
  host: localhost
  grpc_port: 6334
  collections:
    knowledge: knowledge_chunks
    memory: long_term_memories

dashscope:
  api_key: ${DASHSCOPE_API_KEY}
  llm_model: qwen-plus
  embedding_model: text-embedding-v3
  embedding_dimension: 1024

tika:
  url: http://localhost:9998

memory:
  short_term:
    window_size: 10
    slide_step: 4
    summary_max_tokens: 300
    ttl: 24h
  long_term:
    recall_top_k: 3
    score_threshold: 0.75

rag:
  chunking:
    similarity_threshold: 0.3
    min_chunk_size: 100
    max_chunk_size: 1000
  retrieval:
    top_k: 5
    rrf_k: 60
    query_rewrite_count: 3
```

---

## 九、可行性分析总结

### 9.1 技术可行性 ✅

| 维度 | 评估 | 说明 |
|------|------|------|
| LangChainGo 成熟度 | ⭐⭐⭐ | 核心抽象 (LLM/Embedder/VectorStore) 已稳定，但生态不如 Python 版丰富，部分需自行封装 |
| Qdrant 混合检索 | ⭐⭐⭐⭐⭐ | 原生支持 dense+sparse 混合检索，Go gRPC client 官方维护 |
| 阿里百炼 API | ⭐⭐⭐⭐ | 兼容 OpenAI API 格式，LangChainGo 可直接对接 |
| 语义分块 | ⭐⭐⭐ | 需自行实现，Go 中无成熟的开箱即用库，但算法本身不复杂 |
| 中文分词 (BM25) | ⭐⭐⭐⭐ | `go-ego/gse` 基于 jieba 算法，效果可靠 |
| 文档解析 | ⭐⭐⭐⭐ | Tika Server 成熟稳定，Go 原生库覆盖常见格式 |
| 流式输出 | ⭐⭐⭐⭐⭐ | Gin SSE 原生支持，实现简单 |

### 9.2 需要注意的风险点

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| LangChainGo 部分功能缺失 | 中 | 对于 Memory/Retriever 等高级功能，可能需要自行实现接口 |
| 语义分块 Embedding 调用量大 | 中 | 分块是离线操作，可以批量调用 + 限速；考虑缓存已计算的 embedding |
| BM25 IDF 统计需全局数据 | 低 | 初始化时从 Qdrant 加载文档统计，或定时更新 IDF 表 |
| 阿里百炼 API 限流 | 中 | 实现指数退避重试 + 请求队列 |
| 中文语义分块效果 | 中 | 中文句子边界不如英文清晰，需优化句子分割规则 (句号/问号/分号) |

### 9.3 开发排期建议

| 阶段 | 内容 | 预估工时 |
|------|------|---------|
| P0 | 项目骨架 + 用户模块 + 基础对话 (直接调 LLM) | 3 天 |
| P1 | 短期记忆 (滑动窗口 + 摘要 + Redis) | 2 天 |
| P2 | 知识库管理 + 文档解析 + 语义分块 + 入库 | 4 天 |
| P3 | RAG 混合检索 (BM25 + 向量 + RRF) | 3 天 |
| P4 | 查询重写 | 1 天 |
| P5 | 长期记忆 (录入 + 向量化 + 召回) | 2 天 |
| P6 | 流式输出 (SSE) + 前端联调 | 2 天 |
| P7 | 测试 + 优化 + 部署 | 3 天 |
| **合计** | | **约 20 天** |

---

## 十、附录

### A. 阿里百炼 API 兼容 OpenAI 格式

阿里百炼提供了 OpenAI 兼容的 API endpoint，LangChainGo 可直接使用 OpenAI provider 对接：

```go
llm, _ := openai.New(
    openai.WithModel("qwen-plus"),
    openai.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1"),
    openai.WithToken(os.Getenv("DASHSCOPE_API_KEY")),
)

embedder, _ := openai.NewEmbedder(
    openai.WithModel("text-embedding-v3"),
    openai.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1"),
    openai.WithToken(os.Getenv("DASHSCOPE_API_KEY")),
    openai.WithEmbeddingDimension(1024),
)
```

### B. Qdrant 混合检索 API 示例

Qdrant 1.7+ 支持 `prefetch` + `fusion` 模式实现混合检索：

```go
// 使用 Qdrant Go gRPC client
searchResult, _ := client.Query(ctx, &qdrant.QueryPoints{
    CollectionName: "knowledge_chunks",
    Prefetch: []*qdrant.PrefetchQuery{
        {
            Query: qdrant.NewQueryDense(denseVector),  // 语义检索
            Using: qdrant.PtrOf("dense"),
            Limit: qdrant.PtrOf(uint64(20)),
        },
        {
            Query: qdrant.NewQuerySparse(sparseIndices, sparseValues),  // BM25
            Using: qdrant.PtrOf("bm25"),
            Limit: qdrant.PtrOf(uint64(20)),
        },
    },
    Query: qdrant.NewQueryFusion(qdrant.Fusion_RRF),  // RRF 融合
    Limit: qdrant.PtrOf(uint64(5)),
    Filter: &qdrant.Filter{
        Must: []*qdrant.Condition{
            qdrant.NewMatch("knowledge_base_id", knowledgeBaseID),
        },
    },
})
```

### C. Gin SSE 流式输出示例

```go
func (h *ChatHandler) StreamChat(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    streamCh := make(chan string)
    go h.chatService.GenerateStream(c.Request.Context(), req, streamCh)

    c.Stream(func(w io.Writer) bool {
        if chunk, ok := <-streamCh; ok {
            c.SSEvent("message", map[string]string{
                "content": chunk,
                "role":    "assistant",
            })
            return true
        }
        // 发送结束事件
        c.SSEvent("message", map[string]string{
            "content":       "",
            "finish_reason": "stop",
        })
        return false
    })
}
```
