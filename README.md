# InterviewMaster

面向个人求职者的 AI 面试训练平台。用户上传简历、选择主技术语言与目标公司后即可开始后端岗位模拟面试，系统会动态追问并保留可评分、可复盘的完整面试记录。

当前 M0～M3 与 Beta 本地能力均已完成，并通过数据库迁移、真实 HTTP 闭环和 Whisper 音频转写验收。

首发 MVP 聚焦文字模拟面试；题目大纲和候选问题属于后台面试编排，不作为用户管理资源。面经检索与 ASR 位于 `/api/v1/beta`。

## 技术基线

- 后端：Go、go-zero REST、Asynq Worker、PostgreSQL 18 + pgvector、Redis、S3 兼容对象存储。
- 前端：Next.js App Router、React、TypeScript、pnpm。
- 契约：`backend/api/interviewmaster.api` 是 HTTP API 单一事实来源；Go handler、OpenAPI 和 TypeScript SDK 均从它生成。

完整设计见 [技术方案_v1.md](docs/技术方案_v1.md)，本轮产品调整记录见 [上线重构_0814.md](docs/上线重构_0814.md)。

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

分别启动 API、Worker 和 Web：

```powershell
cd backend; go run -buildvcs=false ./apps/api -f apps/api/etc/interviewmaster.yaml
cd backend; go run -buildvcs=false ./apps/worker -f apps/worker/etc/worker.yaml
pnpm --dir web dev
```

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
docs/                 产品、架构、部署与本轮上线重构文档
scripts/              Windows 本地开发脚本
docker-compose.yml    本地依赖及可选完整应用编排
```
