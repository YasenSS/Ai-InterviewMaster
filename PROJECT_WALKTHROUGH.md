# InterviewMaster 项目通读

> 由 project-walkthrough 生成 — 2026-08-06

## 目录
1. 项目定位
2. 整体架构
3. 目录骨架结构
4. 分层讲解
5. 能实现的功能
6. 难点
7. 值得学习的地方

## 1. 项目定位

InterviewMaster 是面向个人求职者的 AI 面试训练平台（monorepo）。

核心闭环：上传并分析简历 → 绑定目标岗位/JD → 生成个性化预测题 → 文字模拟面试 → 输出评分与复盘报告。Beta 能力含面经情报检索与 ASR 语音转写（`/api/v1/beta`）。MVP 聚焦文字模拟面试；M0～M3 与 Beta 本地能力已完成。

技术基线：Go + go-zero REST、Asynq Worker、PostgreSQL 18 + pgvector、Redis、S3 兼容对象存储（MinIO）；前端 Next.js App Router + React + TypeScript + pnpm。HTTP 契约以 `backend/api/interviewmaster.api` 为单一事实源，生成 Go handler、OpenAPI 与 TypeScript SDK。

来源：`README.md`。

## 2. 整体架构

三个应用进程 + 一套基建。浏览器主要面对 Web；大文件经预签名直传 MinIO。

```text
浏览器
  │  REST（同源 /api/v1 → Next rewrite）
  │  预签名 PUT → MinIO
  ▼
Next.js (web/) ──rewrite──► Go API (apps/api)
                              ├── PostgreSQL
                              ├── Redis（缓存 + Asynq 入队）
                              └── MinIO（签名/元数据）
                                    │
                         Asynq 队列 │
                                    ▼
                              Worker (apps/worker)
                              ├── Tika / Whisper(ASR)
                              └── 回写 PG / 读 MinIO
```

| 进程 | 入口 | 职责 |
| --- | --- | --- |
| API | `backend/apps/api/interviewmaster.go` | 配置 → `svc.ServiceContext` → `handler.RegisterHandlers` → `server.Start` |
| Worker | `backend/apps/worker/main.go` | Asynq：`resume:parse` / `object:cleanup` / `asr:transcribe` |
| Web | `web/src/app/` + `web/next.config.ts` | App Router；`/api/v1/*` rewrite 到 API |

API 依赖注入见 `backend/apps/api/internal/svc/servicecontext.go`（DB / Redis / ObjectStore / UploadSigner / TaskClient）。本地基建见 `docker-compose.yml`（postgres、redis、minio、tika；app/beta profiles）。

三条路径：同步 REST；异步任务（API 入队 → Worker）；预签名直传对象存储。

## 3. 目录骨架结构

### 3.1 端到端主线

主线：注册 → 上传简历（异步解析）→ 建 JD → 生成题集 → 创建/答题 → 报告。

```text
register/page → AuthForm → services.register → registerhandler/logic
resumes/new → ResumeUploadPage
  → createResumeUpload（预签名）→ PUT MinIO → completeResumeUpload
  → resume_service Enqueue → Worker resumeparse（Tika + PG）
jobs/new → createJob → createjobdescriptionlogic
question-sets/new → createQuestionSet → createquestionsetlogic
interviews/new|/[id] → createInterview / answerInterview → interview logic
interviews/[id]/report → report → getinterviewreportlogic
```

前端聚合点：`web/src/shared/api/services.ts`。异步分叉：`resume_service.go` → Worker `resumeparse.go`。

### 3.2 目录结构

| 目录 | 输入 | 输出 | 交给谁 |
| --- | --- | --- | --- |
| `backend/api/` | `.api` 契约 | OpenAPI + goctl 产物 | api handler、web TS SDK |
| `backend/apps/api/` | HTTP + ServiceContext | JSON；可选 Asynq 入队 | PG / Redis / MinIO / Worker |
| `backend/apps/worker/` | Asynq payload | 解析/ASR 结果写库 | Tika、ASR、PG、MinIO |
| `backend/internal/platform/` | 配置与连接 | 基建能力 | api + worker |
| `backend/internal/tasks/` | 任务类型定义 | 可入队任务 | api 入队 / worker 消费 |
| `backend/migrations/` | Goose SQL | schema | PostgreSQL |
| `web/src/app/` | 路由 | 薄页面壳 | `features/*` |
| `web/src/features/` | 用户操作 | 调 API、渲染 | `shared/api` |
| `web/src/shared/` | 调用意图 | HTTP client / SDK | API |
| `web/src/components/` | UI 属性 | 复用组件 | features / app |
| `scripts/` / `Makefile` | 开发命令 | generate / check / migrate | 工程流 |
| `docs/` + 根 `*.md` | — | 设计文档 | 人读 |
| `docker-compose.yml` | env | 本地编排 | 基建进程 |

补充：当前无 LLM/Agent 实现；出题/评分/报告多为 stub；真外部服务为 Tika 与 Whisper ASR。设计见 `技术方案_v1.md` §6。

## 4. 分层讲解

（通读时跳过，待补。）要点备忘：契约源为 `backend/api/interviewmaster.api`，`make generate` 生成 Go handler / OpenAPI / TS 类型；API 分层 handler→logic→svc；Worker 消费 Asynq；前端 app→features→shared/api。

## 5. 能实现的功能

| 能力 | 说明 | 实现落点 |
| --- | --- | --- |
| 账号与会话 | 注册/登录/刷新/登出，改资料/改密 | `auth/*`，`features/auth` |
| 工作区隔离 | 用户数据互不可见 | workspace API + DB |
| 简历 | 预签名上传 → Tika 解析；CRUD/重解析 | `resumes/*`，Worker `resumeparse`，`features/resumes` |
| JD | CRUD + 能力标签 | `job-descriptions/*`，`features/jobs` |
| 题集 | 生成/列表/改删/再生（当前固定五题 stub） | `question-sets/*`，`features/question-sets` |
| 文字面试 | 建会话、逐题答/跳过、恢复、完成锁定 | `interviews/*`，`features/interviews` |
| 报告 | 总评+逐题；同会话复用报告 | `getinterviewreportlogic`，`features/reports` |
| 异步任务 | 状态查询与重试 | `tasks/*`，`features/tasks` |
| Dashboard / 探活 | 聚合摘要；health/ready | `dashboard` / `system` |
| Beta 面经 / ASR | 本地降级情报；Whisper 转写 | `/beta/*`，Worker `asr` |

边界：出题/评分/面经多为 stub；真外部智能为 Tika 与 Whisper。对照 `docs/验收清单.md`。

## 6. 难点

| 难点 | 位置 | 为什么难 |
| --- | --- | --- |
| Refresh Token 轮转 | `logic/auth/refreshlogic.go` | `FOR UPDATE` + 新会话插入 + 旧会话 revoke/replaced_by；并发与盗用检测 |
| 简历上传链路 | `logic/workspace/resume_service.go` | 预签名直传 + 服务端验大小/magic bytes + 幂等入队；DB↔Asynq 双写回退 |
| 面试状态机 | `logic/workspace/createinterviewlogic.go` | `lockActiveInterview` 行锁；答题/跳过/完成/计时；未完成确认 |
| 报告一生一次 | `logic/workspace/getinterviewreportlogic.go` | 锁会话 + completed + 同事务生成；`session_id UNIQUE` |
| 租户隔离 | `logic/workspace/helpers.go` 等 | 每条查询自带 `user_id`；漏写即 IDOR，靠纪律 |
| 任务重试 | `logic/workspace/gettasklogic.go` | 失败任务去重再入队；heavy/default 队列 |

工程价值点：会话安全、并发状态机、对象存储+异步解析、幂等报告、双写一致性、契约生成。
