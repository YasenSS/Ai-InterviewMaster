# InterviewMaster Local Setup Guide

> The basic startup flow was verified on 2026-08-15 in a Windows 11, PowerShell, and Docker Desktop environment. The current database version is v11, and the main product flow is “invite-code registration → resume upload → choose language and target company → enter a dynamic backend mock interview → view the complete interview record.”
> See the document index in [`README.md`](README.md); see [`invite-beta-todo_0828.md`](invite-beta-todo_0828.md) for the remaining invite-only work. See [`runbooks/ai.md`](runbooks/ai.md) for model failures.

## 1. Services and addresses

| Service | Purpose | Local address |
| --- | --- | --- |
| Web | Next.js product frontend | <http://localhost:3000> |
| API | go-zero REST API | <http://localhost:8080> |
| PostgreSQL + pgvector | Business data, resume facts, and interview records | `localhost:5432` |
| Redis | Asynq task queue | `localhost:6379` |
| MinIO | Resume and audio object storage | API `localhost:9000`; console <http://localhost:9001> |
| Tika | PDF, DOCX, and text resume parsing | `localhost:9998` |
| Worker | Resume parsing, interview preparation, and report generation | No public port |
| Whisper ASR (optional) | Beta speech transcription | `localhost:9002` |

## 2. Prerequisites

- Docker Desktop is running and can execute `docker ps`.
- A `.env` file exists in the repository root; copy `.env.example` on first startup.
- Local Go, Node.js, and pnpm are needed only when running migrations, generating code, or doing hybrid development.

~~~powershell
Copy-Item .env.example .env
~~~

Do not commit real model keys, database passwords, or object-storage keys to Git.

## 3. First startup or upgrade

### 3.1 Start the infrastructure

Run this from the project root:

~~~powershell
docker compose up -d postgres redis minio tika
docker compose ps
~~~

PostgreSQL and Redis should show `healthy`.

### 3.2 Back up before upgrading from v8

Migration `00009_launch_redesign.sql` deletes the old JD tables. If local data already exists, back it up first:

~~~powershell
New-Item -ItemType Directory -Force .cache/backups | Out-Null
docker exec interviewmaster-postgres-1 sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc -f /tmp/before_v9.dump'
docker cp interviewmaster-postgres-1:/tmp/before_v9.dump .cache/backups/before_v9.dump
~~~

The backup actually created for this upgrade was:

~~~text
.cache/backups/before_launch_redesign_20260814_0147.dump
~~~

`00010_agent_runtime_v2.sql` adds Agent runtime, decision, and question-snapshot tables on top of v9. It does not restore JD data deleted by v9, so an old environment must still complete the backup above before running v9.

### 3.3 Run database migrations

`goose` does not automatically read the root `.env`. Replace the connection string below with the actual user, password, and database name used in `.env`:

~~~powershell
cd backend
$env:IM_DATABASE_URL = "postgres://<POSTGRES_USER>:<POSTGRES_PASSWORD>@localhost:5432/<POSTGRES_DB>?sslmode=disable"
go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir migrations postgres $env:IM_DATABASE_URL up
cd ..
~~~

Check migration status:

~~~powershell
cd backend
go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir migrations postgres $env:IM_DATABASE_URL status
cd ..
~~~

The latest version should be `00011_invite_codes.sql`.

### 3.4 Create a local invite code

The registration API uniformly requires an invite code. After the migration finishes, run the following command from the repository root to create a local invite code that can be used 10 times. Replace the example value with your own short-lived test code; do not put production invite codes in Git:

~~~powershell
$inviteCode = "LOCAL-CHANGE-ME"
docker compose exec -T postgres psql -U interviewmaster -d interviewmaster -c "INSERT INTO invite_codes (code_hash, max_uses) VALUES (encode(digest('$inviteCode', 'sha256'), 'hex'), 10);"
~~~

Enter the same invite code on the registration page. Production invite codes must be issued, revoked, and recorded through controlled database operations; they must not be written to the repository or public logs.

## 4. Recommended startup method: all-container mode

For the first build, after code changes, or after Docker Desktop restarts when the whole project needs to be restored:

~~~powershell
docker compose --profile app up -d --build api worker web
~~~

This command also starts the PostgreSQL, Redis, MinIO, and Tika services on which the application depends. If you only want to restore all services using existing images, run:

~~~powershell
docker compose --profile app up -d
~~~

If only the infrastructure has been started and the application layer should be started afterward:

~~~powershell
docker compose --profile app up -d api worker web
~~~

View the status:

~~~powershell
docker compose --profile app ps
~~~

Normally, the following services should be running:

- `postgres`, `redis`, `minio`, `tika`
- `api`, `worker`, `web`

The Web image runs in Next.js standalone mode. `.dockerignore` excludes local `node_modules`, `.next`, and cache directories so that Windows dependencies do not overwrite container dependencies.

## 5. Startup verification

PowerShell:

~~~powershell
Invoke-WebRequest -UseBasicParsing http://localhost:8080/api/v1/health
Invoke-WebRequest -UseBasicParsing http://localhost:8080/api/v1/ready
Invoke-WebRequest -UseBasicParsing http://localhost:3000/
Invoke-WebRequest -UseBasicParsing http://localhost:3000/api/v1/health
~~~

Expected results:

- API `/health` returns HTTP 200.
- API `/ready` returns HTTP 200, with both `database` and `redis` equal to `ok`.
- The Web homepage returns HTTP 200.
- Accessing `/api/v1/health` through Web also returns HTTP 200, proving that the internal container proxy works.

Open <http://localhost:3000> in a browser. Use a valid invite code to register, or log in directly if you already have an account. The frontend Mock is only for viewing the page and does not validate real invite codes.

## 6. Logs and troubleshooting

View recent logs:

~~~powershell
docker compose logs --tail 100 api worker web
~~~

Follow logs continuously:

~~~powershell
docker compose logs -f api worker web
~~~

Rebuild only one service:

~~~powershell
docker compose --profile app up -d --build api
docker compose --profile app up -d --build worker
docker compose --profile app up -d --build web
~~~

If the Web homepage works but `/api/v1/*` returns 500, confirm that the Web image is up to date and rebuild it:

~~~powershell
docker compose --profile app up -d --build web
~~~

If `dockerDesktopLinuxEngine` reports that the pipe does not exist, the Docker engine has not started. Open Docker Desktop, wait for `docker info` to succeed, and then run the Compose command again. Do not delete data volumes to handle this issue.

## 7. Hybrid development mode

When code hot reload is needed, run only the infrastructure in Docker and start the three application processes in three separate terminals.

First load the root `.env` into the current PowerShell process:

~~~powershell
Get-Content .env | ForEach-Object {
    if ($_ -match '^\s*([^#][^=]*)=(.*)$') {
        $name = $matches[1].Trim()
        $value = $matches[2].Trim().Trim('"').Trim("'")
        Set-Item -Path "Env:$name" -Value $value
    }
}
~~~

Then run:

~~~powershell
# Terminal 1: API
cd backend
go run -buildvcs=false ./apps/api -f apps/api/etc/interviewmaster.yaml

# Terminal 2: Worker
cd backend
go run -buildvcs=false ./apps/worker -f apps/worker/etc/worker.yaml

# Terminal 3: Web
pnpm --dir web dev
~~~

Do not start the local Web or API again while all-container mode is still occupying ports 3000 and 8080.

## 8. AI mode

When the root `.env` uses `IM_AI_ENABLED=false`, the complete flow can run without model keys: the Worker generates rule-based internal materials, advances dynamically to the target number of turns according to server policy, and the report clearly marks the result as a degraded evaluation. Rule mode is not real-time model scoring; the page displays “Rule-degraded interview” and does not present it as an AI result.

When enabling a model, configure `.env` as follows:

~~~dotenv
IM_AI_ENABLED=true
IM_AI_PROVIDER=openai-compatible
IM_AI_BASE_URL=https://api.openai.com/v1
IM_AI_API_KEY=replace-with-real-key
IM_AI_CHAT_MODEL=gpt-4.1-mini
IM_AI_REQUEST_TIMEOUT=30s
IM_AI_MAX_OUTPUT_TOKENS=4096
IM_AI_STRUCTURED_OUTPUTS=true
~~~

Once enabled:

- The Worker generates internal blueprints and materials from the resume-version snapshot, primary language, fixed backend role, and target company.
- After each user answer, the API only persists the answer and decision task and returns HTTP 202. The Worker then makes a bounded decision among follow-up, switching focus, and finishing; the frontend polls the business state.
- The standard policy is at least 8 turns, a target of 12, at most 16, and approximately 30 minutes total. The server also limits follow-up depth and budget; the interview is no longer fixed at five questions.
- Each decision writes exactly one next-turn question to the database; internal materials not shown to the user remain on the backend.
- After the interview completes, the Worker scores each turn and generates the full review.

Internal candidate questions and generic asynchronous tasks are not exposed as frontend management modules.

When using DeepSeek V4, configure:

~~~dotenv
IM_AI_ENABLED=true
IM_AI_PROVIDER=openai-compatible
IM_AI_BASE_URL=https://api.deepseek.com
IM_AI_API_KEY=replace-with-real-key
IM_AI_CHAT_MODEL=deepseek-v4-pro
IM_AI_SMALL_MODEL=deepseek-v4-pro
IM_AI_EMBEDDING_MODEL=
IM_AI_STRUCTURED_OUTPUTS=true
~~~

The compatibility layer uses `json_object` for OpenAI-compatible models, while the application still performs JSON Schema and domain validation internally. Real-time interview calls for DeepSeek V4 disable default deep thinking to avoid repeated 30-second timeouts during capability planning and per-turn decisions. After changing `.env`, rebuild or recreate the API and Worker, and create a new interview; historical `agent_mode=rule` sessions are not rewritten.

## 9. Optional ASR

~~~powershell
docker compose --profile beta up -d asr
~~~

The first startup may need to download a Whisper model. ASR does not affect the main text-interview flow.

## 10. Stop and recover

Stop only the application layer while keeping the infrastructure:

~~~powershell
docker compose --profile app stop api worker web
~~~

Start the application layer again:

~~~powershell
docker compose --profile app start api worker web
~~~

Stop all containers while retaining database volumes:

~~~powershell
docker compose --profile app down
~~~

Do not run `docker compose down -v` unless you explicitly want to delete all local database and object-storage data.

## 11. Actual acceptance results on 2026-08-15

- Docker Desktop was initially closed, then restarted; Docker Engine `29.1.2` was confirmed available.
- API, Worker, and Web images were rebuilt from the current workspace code instead of reusing old code containers.
- Database migration status was confirmed at v10: `00010_agent_runtime_v2.sql`.
- API, Worker, Web, PostgreSQL, Redis, MinIO, and Tika all remained running.
- `GET http://localhost:8080/api/v1/health`: HTTP 200.
- `GET http://localhost:8080/api/v1/ready`: HTTP 200; both the database and Redis were `ok`.
- `GET http://localhost:3000/`: HTTP 200.
- `GET http://localhost:3000/api/v1/health`: HTTP 200; the internal Web-to-API proxy worked.
- API, Worker, and Web startup logs showed no crashes or connection errors; the Worker had begun consuming Asynq tasks.
- The local environment was connected to `deepseek-v4-pro`: real model calls completed for capability blueprint, internal interview materials, and next-turn follow-ups; the verification session used `agent_mode=ai`, and both the first question and follow-up had `generation_mode=ai`.
