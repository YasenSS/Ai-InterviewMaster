# InterviewMaster Phase 0 & Phase 1 Development Schedule and Task Breakdown

> **Archived.** Historical scheduling draft. Its OceanBase and multi-service tasks are no longer the basis for the current schedule; see [`technical-plan_v1.md`](technical-plan_v1.md) for the historical engineering baseline.

## Overview

Based on the InterviewMaster full-development outline, this document provides a detailed task breakdown, effort estimate, and dependency plan specifically for **Phase 0 (project infrastructure and technology-selection implementation)** and **Phase 1 (microservice decomposition and interface contracts)**.

The two phases were estimated to take approximately **4–7 working days** in total. The goal was to take the project from a blank starting point through infrastructure setup, microservice skeleton generation, and a minimal end-to-end MVP skeleton.

---

## Phase 0: Project Infrastructure and Technology Selection (Estimated: 1–3 days)

**Phase goal**: Establish a unified Monorepo structure and package shared foundational components (logging, configuration, database connections, unified responses, and so on), so that subsequent microservice development has a consistent base.

### 0.1 Repository Initialization and Engineering Standards (0.5–1 day)

- **[Task 0.1.1] Initialize the Monorepo repository**
  - **Details**: Initialize the Git repository and establish a standard directory structure: `apps/` (microservices), `api/` (gateway API), `rpc/` (gRPC Proto), `pkg/` (shared foundational components), and `deploy/` (deployment configuration).
  - **Deliverables**: A clear directory skeleton, a basic `.gitignore`, and an initialized `go.mod`.
- **[Task 0.1.2] go-zero environment and automation scripts**
  - **Details**: Write a basic `Makefile` toolchain with standardized commands such as `make gen-api` / `make gen-rpc` to generate scaffolding from definition files.
  - **Deliverables**: A Makefile that can generate API/RPC code with one command.

### 0.2 Shared Component Library (`pkg`) Development (1–1.5 days)

- **[Task 0.2.1] OceanBase/DB client wrapper**
  - **Details**: Wrap database connections using GORM or the official driver, with support for multi-tenant context propagation, connection-pool configuration, and unified logging.
  - **Deliverables**: The `pkg/db` component package.
- **[Task 0.2.2] Authentication middleware and JWT components**
  - **Details**: Implement JWT token generation and parsing, with authentication interceptors for both HTTP (gateway layer) and RPC (internal services).
  - **Deliverables**: The `pkg/auth` component package.
- **[Task 0.2.3] Observability component wrapper**
  - **Details**: Integrate a structured logging library such as zap, inject and propagate the TraceId through HTTP/RPC/Asynq, and connect basic Prometheus metrics.
  - **Deliverables**: The `pkg/logger` and `pkg/trace` component packages.
- **[Task 0.2.4] Unified API responses and error-code conventions**
  - **Details**: Define global standard error codes, distinguishing business errors from system errors, together with a standard JSON response format at the API layer.
  - **Deliverables**: The `pkg/errors` package and a unified response wrapper.

### 0.3 Local Infrastructure Deployment (0.5 day)

- **[Task 0.3.1] Set up local infrastructure with docker-compose**
  - **Details**: Write `docker-compose.yml` to start the complete set of local dependencies with one command: OceanBase (or MySQL temporarily as a substitute) and Redis (for cache and queue dependencies).
  - **Deliverables**: A complete local dependency environment that can be started with `docker-compose up -d`.

---

## Phase 1: Microservice Decomposition and Interface Contracts (Estimated: 3–7 days)

**Phase goal**: Use go-zero to decompose the core microservices, define API and RPC contracts for each service, and connect the basic path of “frontend request → gateway routing → RPC processing → asynchronous task enqueueing and consumption.”

### 1.1 API Gateway (`gateway-api`) Setup (0.5–1 day)

- **[Task 1.1.1] Define the gateway contract (`.api`)**
  - **Details**: Write `gateway.api` to define the RESTful APIs required by the frontend, including login/registration, resume upload, and review triggers.
  - **Deliverables**: The `api/gateway/gateway.api` contract document and the generated Go HTTP service skeleton.
- **[Task 1.1.2] Integrate gateway middleware**
  - **Details**: Apply cross-origin (CORS) configuration, request-rate limiting, and the `pkg/auth` authorization middleware at the HTTP layer.

### 1.2 User Service (`user-svc`) MVP (0.5–1 day)

- **[Task 1.2.1] Generate the user-service `.proto` contract and skeleton**
  - **Details**: Define basic RPC interfaces for login, registration, and authenticated-user retrieval, then use go-zero to generate the RPC service.
  - **Deliverables**: `rpc/user/user.proto` and its corresponding server stubs.
- **[Task 1.2.2] Implement basic authentication business logic**
  - **Details**: Implement simple user registration and login, connect to the `im_user` table, and verify JWT issuance.

### 1.3 Resume Service (`resume-svc`) Contract and Upload Flow (1–2 days)

- **[Task 1.3.1] Define the resume-service `.proto` contract**
  - **Details**: Define interfaces for resume upload, status queries, and parsing triggers.
  - **Deliverables**: `rpc/resume/resume.proto`.
- **[Task 1.3.2] Store resume uploads and metadata**
  - **Details**: Connect the gateway upload entry point, temporarily store files or connect to object storage, and create an initial `im_resume` record in the database with status `pending`.
- **[Task 1.3.3] Enqueue asynchronous parsing tasks**
  - **Details**: Integrate the Asynq Client; enqueue a parsing task after resume upload completes and return a task_id to the caller.

### 1.4 AI Orchestration Skeleton (`rag-svc` & `interview-svc`) (1 day)

- **[Task 1.4.1] Define AI-core orchestration `.proto` contracts**
  - **Details**: Define RAG-based RPC contracts such as `Predict` (question prediction) and `Critique` (review critique).
  - **Deliverables**: `rpc/rag/rag.proto` and `rpc/interview/interview.proto`.
- **[Task 1.4.2] Stub code and mock responses**
  - **Details**: Because AI logic would be implemented in later phases, only write stub code at this stage, connect RPC calls, validate parameter passing, and return fixed mock structures for frontend/backend integration.

### 1.5 Asynchronous Task Engine (`ingestion-worker`) Integration (1 day)

- **[Task 1.5.1] Initialize the Asynq consumer service**
  - **Details**: Start an independent Asynq consumer service under `apps/ingestion` and register multiple-priority queues such as `q.realtime`, `q.batch`, and `q.heavy`.
- **[Task 1.5.2] Register a mock resume-parsing handler**
  - **Details**: Write Worker handling logic to consume resume-parsing tasks from `resume-svc`, simulate processing latency, and update the database status to `completed`.

### 1.6 End-to-End Integration and Acceptance (0.5–1 day)

- **[Task 1.6.1] Test the “upload → process → query” loop**
  - **Details**: Simulate the real request path: send an authenticated request through the gateway → trigger the resume-upload RPC → enqueue an Asynq task → let the Worker finish and update the database → query the completed status through the gateway.
- **[Task 1.6.2] Observability acceptance**
  - **Details**: Verify that a request’s TraceId is correctly carried from the HTTP header through RPC and consistently appears in Asynq task logs, ensuring troubleshootability.

---

## Reference Execution Timeline (Estimated Gantt Chart)

- **Day 1**: Startup phase. Complete the 0.1 repository initialization and directory split; set up the 0.3 Docker Compose environment.
- **Day 2**: Component packaging. Concentrate on the 0.2 shared components (DB, Auth, Logger, Trace, unified responses), and integrate them with the scaffolding for validation.
- **Day 3**: Contract first. Write all `.api` and `.proto` contract files (1.1.1, 1.2.1, 1.3.1, 1.4.1) and generate the complete service skeleton with the tools.
- **Day 4**: Gateway and user layer. Implement gateway CORS and authentication (1.1.2) and complete the User Service authentication loop (1.2.2).
- **Day 5**: Resume flow. Implement Resume Service upload persistence (1.3.2) and asynchronous task enqueueing (1.3.3).
- **Day 6**: Asynchronous engine. Complete the Ingestion Worker and connect the asynchronous status flow with the resume service (1.5).
- **Day 7**: Integration and acceptance. Implement mock responses for RAG and Interview services (1.4.2), complete end-to-end integration tests, Trace verification, and code review (1.6).
