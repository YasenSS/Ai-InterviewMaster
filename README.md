[English](#interviewmaster) | [中文](#interviewmaster中文)

# InterviewMaster

An AI interview training platform for individual job seekers. Invited users register with an invitation code, upload a resume, choose a primary technical language and target company, and start a backend interview simulation with dynamic follow-up questions and a complete, scoreable review record.

The M0–M3 and local Beta capabilities are complete, including database migrations, the real HTTP loop, and Whisper audio transcription validation.

The initial MVP focuses on text-based mock interviews. Interview blueprints and candidate questions are internal orchestration resources rather than user-managed assets. Interview intelligence search and ASR are available under `/api/v1/beta`.

## Technical baseline

- Backend: Go, go-zero REST, Asynq Worker, PostgreSQL 18 + pgvector, Redis, and S3-compatible object storage.
- Frontend: Next.js App Router, React, TypeScript, and pnpm.
- Contract: `backend/api/interviewmaster.api` is the single source of truth for the HTTP API; Go handlers, OpenAPI, and the TypeScript SDK are generated from it.

See the [English documentation index](docs/en/README.md). The product line is described in [Launch redesign](docs/en/launch-redesign_0814.md), and the remaining invite-beta work is in [Invite beta TODO](docs/en/invite-beta-todo_0828.md). The historical technical baseline is in `docs/en/archive/`.

## Local setup

Prepare the environment file:

```powershell
Copy-Item .env.example .env
docker compose up -d postgres redis minio tika
```

Run database migrations:

```powershell
cd backend
$env:IM_DATABASE_URL = "postgres://interviewmaster:interviewmaster_local_only@localhost:5432/interviewmaster?sslmode=disable"
go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir migrations postgres "$env:IM_DATABASE_URL" up
```

Registration requires an invitation code. See the [local setup guide](docs/en/local-setup.md#34-create-a-local-invitation-code) for how to create one after the first migration.

Start the API, Worker, and Web separately:

```powershell
cd backend; go run -buildvcs=false ./apps/api -f apps/api/etc/interviewmaster.yaml
cd backend; go run -buildvcs=false ./apps/worker -f apps/worker/etc/worker.yaml
pnpm --dir web dev
```

To view the frontend without starting the backend, use the Mock environment (port **3001**, separate from the normal development port 3000):

```powershell
pnpm mock:web
```

Any email and password can be used in Mock mode. The upper-right corner displays a Mock badge.

You can also start the complete application stack in containers:

```powershell
docker compose --profile app up --build
```

Start the local Whisper Beta ASR service:

```powershell
docker compose --profile beta up -d asr
```

## Common commands

```powershell
# Generate code from the API contract and SQL
.\scripts\generate.ps1

# Run backend/frontend checks, tests, and builds
.\scripts\check.ps1

# Install frontend dependencies
pnpm install
```

After starting the API, visit `GET http://localhost:8080/api/v1/health` and `GET http://localhost:8080/api/v1/ready`; the default Web address is `http://localhost:3000`.

## Directory

```text
backend/              Go API, Worker, migrations, and SQL queries
web/                  Next.js frontend
docs/                 Localized documentation under en/ and ch/
scripts/              Windows development scripts
docker-compose.yml    Local dependencies and optional full application stack
```

---

# InterviewMaster（中文）

面向个人求职者的 AI 面试训练平台。受邀用户使用邀请码注册后，上传简历、选择主技术语言与目标公司即可开始后端岗位模拟面试，系统会动态追问并保留可评分、可复盘的完整面试记录。

当前 M0～M3 与 Beta 本地能力均已完成，并通过数据库迁移、真实 HTTP 闭环和 Whisper 音频转写验收。

首发 MVP 聚焦文字模拟面试；题目大纲和候选问题属于后台面试编排，不作为用户管理资源。面经检索与 ASR 位于 `/api/v1/beta`。

## 技术基线

- 后端：Go、go-zero REST、Asynq Worker、PostgreSQL 18 + pgvector、Redis、S3 兼容对象存储。
- 前端：Next.js App Router、React、TypeScript、pnpm。
- 契约：`backend/api/interviewmaster.api` 是 HTTP API 单一事实来源；Go handler、OpenAPI 和 TypeScript SDK 均从它生成。

文档目录见 [中文文档索引](docs/ch/README.md)。产品主线见 [上线重构_0814.md](docs/ch/上线重构_0814.md)，邀请制剩余项见 [邀请制todo_0828.md](docs/ch/邀请制todo_0828.md)；历史技术基线已移入 `docs/ch/archive/`。

## 本地启动

先准备环境文件：

```powershell
Copy-Item .env.example .env
docker compose up -d postgres redis minio tika
```

执行数据库迁移：

```powershell
cd backend
$env:IM_DATABASE_URL = "postgres://interviewmaster:interviewmaster_local_only@localhost:5432/interviewmaster?sslmode=disable"
go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir migrations postgres "$env:IM_DATABASE_URL" up
```

注册需要邀请码；首次启动后的创建方式见 [本地启动指南](docs/ch/本地启动.md#34-创建本地邀请码)。

分别启动 API、Worker 和 Web：

```powershell
cd backend; go run -buildvcs=false ./apps/api -f apps/api/etc/interviewmaster.yaml
cd backend; go run -buildvcs=false ./apps/worker -f apps/worker/etc/worker.yaml
pnpm --dir web dev
```

只想看前端页面、不启动后端时，用 Mock 环境（端口 **3001**，与正式开发的 3000 分开）：

```powershell
pnpm mock:web
```

任意邮箱和密码都可以登录。右上角会显示 Mock 标识。

也可以启动完整应用容器：

```powershell
docker compose --profile app up --build
```

启动本地 Whisper Beta ASR：

```powershell
docker compose --profile beta up -d asr
```

## 常用命令

```powershell
# 根据 API 契约和 SQL 生成代码
.\scripts\generate.ps1

# 后端和前端静态检查、测试、构建
.\scripts\check.ps1

# 前端依赖安装
pnpm install
```

运行 API 后可访问 `GET http://localhost:8080/api/v1/health` 与 `GET http://localhost:8080/api/v1/ready`；Web 默认地址为 `http://localhost:3000`。

## 目录

```text
backend/              Go API、Worker、迁移和 SQL 查询
web/                  Next.js 前端
docs/                 中英文文档分别位于 docs/en/ 与 docs/ch/
scripts/              Windows 本地开发脚本
docker-compose.yml    本地依赖及可选完整应用编排
```
