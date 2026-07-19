GOCTL_VERSION := v1.10.1
SQLC_VERSION := v1.31.1
GOOSE_VERSION := v3.27.1
API_FILE := backend/api/interviewmaster.api

.PHONY: help bootstrap generate generate-api generate-sql fmt lint test build web-install web-check infra-up infra-down migrate migrate-status check

help:
	@echo "InterviewMaster development commands"
	@echo "  make bootstrap     Install dependencies and generate code"
	@echo "  make generate      Generate Go/TypeScript API code and sqlc queries"
	@echo "  make check         Format, lint, test, and build the project"
	@echo "  make infra-up      Start PostgreSQL, Redis, MinIO, and Tika"
	@echo "  make migrate       Apply PostgreSQL migrations"

bootstrap: generate web-install
	cd backend && go mod tidy

generate: generate-api generate-sql

generate-api:
	cd backend && go run github.com/zeromicro/go-zero/tools/goctl@$(GOCTL_VERSION) api validate --api api/interviewmaster.api
	cd backend && go run github.com/zeromicro/go-zero/tools/goctl@$(GOCTL_VERSION) api go --api api/interviewmaster.api --dir apps/api
	cd backend && go run github.com/zeromicro/go-zero/tools/goctl@$(GOCTL_VERSION) api swagger --api api/interviewmaster.api --dir api/openapi --yaml
	cd backend && go run github.com/zeromicro/go-zero/tools/goctl@$(GOCTL_VERSION) api ts --api api/interviewmaster.api --dir ../web/src/shared/api/generated
	node -e "require('fs').copyFileSync('web/src/shared/api/gocliRequest.template.ts', 'web/src/shared/api/generated/gocliRequest.ts')"

generate-sql:
	cd backend && go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

fmt:
	cd backend && go fmt ./...
	pnpm --dir web format

lint:
	cd backend && go vet ./...
	pnpm --dir web lint
	pnpm --dir web typecheck

test:
	cd backend && go test ./...
	pnpm --dir web test

build:
	cd backend && go build -buildvcs=false ./apps/api ./apps/worker
	pnpm --dir web build

web-install:
	pnpm install

web-check:
	pnpm --dir web lint
	pnpm --dir web typecheck
	pnpm --dir web test
	pnpm --dir web build

infra-up:
	docker compose up -d postgres redis minio tika

infra-down:
	docker compose down

migrate:
	cd backend && go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir migrations postgres "$$IM_DATABASE_URL" up

migrate-status:
	cd backend && go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir migrations postgres "$$IM_DATABASE_URL" status

check: lint test build
