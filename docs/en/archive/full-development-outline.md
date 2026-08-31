# InterviewMaster Full-Process Development Outline (go-zero + OceanBase + LangChainGo + Asynq)

> **Archived.** Historical technical draft; do not use it as the current development baseline. For historical architecture and technology choices, see [`technical-plan_v1.md`](technical-plan_v1.md) in the same directory.

> Goal: build an extensible RAG system and Go microservice architecture around three core capabilities—**deep review and critique of interview records**, **precise question prediction based on resumes**, and **web-wide interview-experience retrieval**—while balancing privacy, security, observability, and engineering maintainability.

---

## Phase 0: Project Foundations and Technology Selection (1–3 days)

### 0.1 Suggested repository and engineering structure

- **Repository shape**: a multi-service monorepo, so standards, shared proto files, and shared libraries (`pkg`) can be managed consistently.
- **Suggested directories**
  - `apps/`: individual services (`user`, `resume`, `interview`, `search`, `rag`, `ingestion`, `gateway`)
  - `api/`: HTTP API definitions and gateway configuration
  - `rpc/`: service proto files
  - `pkg/`: shared libraries (OceanBase client, Embedding client, LLM client, authentication, logging, tracing, error codes)
  - `deploy/`: k8s/compose/helm

### 0.2 Foundation capabilities (must come first)

- **Unified identity and authentication**: JWT (user-facing) + service-to-service authentication (mTLS or internal signatures).
- **Observability**: structured logs + TraceId propagation across HTTP/RPC/Asynq + metrics (QPS/latency/failure rate/queue backlog).
- **Data security**: encrypted storage for resumes/interview records (field-level/column-level) + strict multi-tenant isolation (Tenant/User dimensions).
- **Prompt and model governance**: versioned prompts, configurable parameters (`temperature/top_p`), and structured output (JSON Schema).

---

## Phase 1: Microservice Decomposition and API Contracts (MVP skeleton, 3–7 days)

### 1.1 Suggested microservice decomposition (go-zero compatible)

#### 1) `gateway-api` (HTTP)

- **Responsibilities**: unified external REST/HTTP; authentication, rate limiting, and lightweight orchestration; file-upload entry point (resume/audio/text).
- **Output**: stable APIs for the frontend; internal calls go through RPC.

#### 2) `user-svc` (RPC)

- **Responsibilities**: users, membership/quotas, privacy settings (whether external interview experience may be used for training/retrieval), API key binding, and so on.

#### 3) `resume-svc` (RPC)

- **Responsibilities**: resume upload, parsing, structured extraction (tech stack/projects/responsibilities/highlights/career gaps), versioning, and triggering embedding generation.
- **External capabilities**: resume knowledge-graph results and resume point-of-interest weights.

#### 4) `interview-svc` (RPC)

- **Responsibilities**: interview sessions, question–answer pairs (QA), scores, critique reports, and Golden Answer generation and versioning.
- **External capabilities**: review-report queries and comparison reports (reference answer vs. actual answer).

#### 5) `search-svc` (RPC)

- **Responsibilities**: external retrieval (Tavily/Firecrawl), webpage crawling and cleaning, and ingestion of company/role-specific interview-experience materials.
- **External capabilities**: retrieval by company/role and material updates.

#### 6) `rag-svc` (RPC, core AI orchestration)

- **Responsibilities**: unified orchestration for three RAG pipelines (resume question prediction, company interview-experience retrieval, and interview critique/optimization).
- **Principle**: decouple business-process orchestration from data ingestion/parsing/retrieval details; `rag-svc` only composes and makes decisions.

#### 7) `ingestion-worker` (Asynq Worker; may be deployed with a service)

- **Responsibilities**: all time-consuming tasks (parsing, embedding, interview-experience crawling, long-text critique, ASR transcription, and so on).
- **Principle**: decouple it from online services to avoid blocking and resource contention.

> Optional enhancements (split out in the full release): `billing-svc` (quota billing), `model-gateway` (multi-model routing/fallback), and `kb-svc` (more general knowledge-base management).

### 1.2 API/RPC contracts (recommend starting with the “minimum closed loop”)

- **Resume**
  - `POST /v1/resumes` upload → return `resume_id`
  - `POST /v1/resumes/{id}/analyze` trigger analysis (async) → return `task_id`
  - `GET /v1/resumes/{id}` retrieve structured results and status
- **Question prediction**
  - `POST /v1/predictions` (`resume_id + target_role/jd` optional) → return a question set (possibly async)
- **Interview-experience retrieval**
  - `POST /v1/company-intel/search` (`company+role+keywords`) → return clusters and frequent questions (possibly refreshed asynchronously)
- **Review**
  - `POST /v1/interviews` create a session (`resume_id + jd`)
  - `POST /v1/interviews/{id}/transcript` upload a text/audio reference
  - `POST /v1/interviews/{id}/critique` trigger critique (async) → `task_id`
  - `GET /v1/interviews/{id}/report` retrieve the report

---

## Phase 2: OceanBase Data Architecture (Unified relational + vector storage, 5–10 days)

> Goal: use one data model to support both **business metadata** (strong consistency, queryability, auditability) and **vector retrieval** (semantic recall), with strict multi-tenant isolation.

### 2.1 Key design principles

- **Multi-tenant isolation**: every table must include `tenant_id` and `user_id` (or `owner_id`) and have a composite index.
- **Three layers—document, chunk, vector**: Document (original/cleaned text) → Chunk (smallest retrievable unit) → Embedding (vector and model version).
- **Traceability and recomputability**: record embedding/chunking/prompt/model versions for recomputation and comparison.
- **Hot/cold separation**: online retrieval uses chunk+embedding; large original text/audio is stored only as references and object-storage addresses.

### 2.2 Suggested business metadata tables

#### User and project domain

- `im_user`: user basics, quota, and privacy switches
- `im_user_setting`: model preference, language preference, whether external interview-experience fusion is enabled, and so on

#### Resume domain

- `im_resume`: resume master table (`resume_id`, `user_id`, `file_url`, `status`, `hash`, `created_at`)
- `im_resume_version`: versioning (multiple updates to the same resume)
- `im_resume_fact`: structured facts (tech stack, projects, responsibilities, metrics); JSON storage plus redundant indexes on key fields is recommended

#### JD/role domain (optional but strongly recommended)

- `im_job_desc`: `jd_id`, `company`, `role`, `content`, `created_at`

#### Interview domain

- `im_interview_session`: `session_id`, `user_id`, `resume_id`, `jd_id`, `status`
- `im_interview_qa`: `qa_id`, `session_id`, `question`, `answer`, `order_no`
- `im_interview_report`: scoring/critique/optimization results (structured JSON with version fields)

#### Interview-experience domain (public material library)

- `im_company_source`: source (site/URL/crawl time/availability)
- `im_company_doc`: document-level metadata (company/role/time_range/author, and so on)

### 2.3 Generalized chunk and vector tables

#### Document table (unified support for resume/transcript/company-intel)

- `im_doc`
  - Key fields: `doc_id`, `tenant_id`, `owner_type` (user/public), `owner_id` (user_id or 0), `doc_type` (resume/transcript/company_intel/jd), `source_ref` (file_url/url/session_id, and so on), `language`, `status`, `created_at`

#### Chunk table (smallest retrievable unit)

- `im_doc_chunk`
  - Key fields: `chunk_id`, `doc_id`, `chunk_no`, `content`, `content_tokens`, `metadata_json` (company, role, project name, time, and so on), `created_at`
  - Indexes: `(tenant_id, owner_type, owner_id, doc_type)`, `(doc_id, chunk_no)`

#### Vector table (decouple embeddings from chunks to support multiple models)

- `im_chunk_embedding`
  - Key fields: `embedding_id`, `chunk_id`, `embed_model`, `embed_dim`, `vector` (OceanBase vector-column type or a compatible vector-index solution), `created_at`
  - Indexes: vector index + `(embed_model, doc_type, owner_type, owner_id)`

> Practical suggestion: isolate the “user-private vector space” and “public interview-experience vector space” in the same table using `owner_type/owner_id`; retrieval must always append filter conditions to prevent unauthorized recall.

### 2.4 Retrieval strategy (multi-path recall)

- **Semantic retrieval**: vector TopK (private resume/transcript + public company_intel)
- **Keyword retrieval (optional enhancement)**: create a full-text index/BM25 on `im_doc_chunk.content` (if the OceanBase environment supports it; otherwise start with simple LIKE/inverted-index service)
- **Reranking (optional)**: introduce a reranker (LLM or lightweight cross-encoder) for secondary ranking of TopK; defer to Phase 3+

---

## Phase 3: Three RAG Pipeline Designs (core business loops, 7–14 days)

### 3.1 RAG pipeline A: Resume analysis and question prediction (Resume-Based Q&A Prediction)

#### A1. Data preparation

- **Input**: PDF/Word resume
- **Processing**: parse → clean → structured extraction (tech stack/projects/metrics/responsibilities/career gaps) → chunk → embedding
- **Outputs**
  - `im_resume_fact`: structured facts (for “do not fabricate” constraints)
  - `im_doc`/`im_doc_chunk`/`im_chunk_embedding`: semantic retrieval (project details, description paragraphs, and so on)

#### A2. Retrieval and generation

- **Query construction**: generate multiple subqueries by “role/direction/JD” (tech stack, project deep-dive points, risk points)
- **Recall**: prioritize TopK recall from the user’s private resume chunks; supplement with public interview experience for the same role as “interview style” context when necessary
- **Generation**: output a structured question set
  - 3–5 progressively deeper follow-ups for each project
  - Each question includes the assessment point, expected answer points, and follow-up branches

#### A3. Quality control (required for MVP)

- **Hallucination Check**: every sentence that “cites a project fact” must be traceable to a `chunk_id` or `resume_fact`; otherwise downgrade it to “suggested wording” and mark the uncertainty.

### 3.2 RAG pipeline B: Web-wide interview-experience retrieval and clustering (Company-Specific Intel)

#### B1. Data ingestion

- **Trigger**: when the user enters company+role, return “cached/historical results” immediately and refresh asynchronously at the same time
- **External retrieval**: use Tavily Search to obtain candidate links → use Firecrawl to crawl the body → clean and remove noise (navigation/comments, and so on)
- **Chunking strategy**: split by “question/answer/paragraph”; retain company/role/time_range/source_url metadata
- **Storage**: write to `im_company_doc` + `im_doc*` (`owner_type=public`)

#### B2. Retrieval and clustering

- **Recall**: vector-retrieve TopK after filtering by company/role, then apply time-decay weighting (higher weight for the past six months)
- **Clustering**: cluster question snippets from TopK by topic (LLM-based “topic merging” is acceptable for MVP; vector clustering can replace it in the full release)
- **Output**
  - frequent-question list (with frequency/source count)
  - interview-style profile (algorithm/fundamentals/scenario/system-design proportions)
  - continuous updates (save clustering-result versions)

### 3.3 RAG pipeline C: Interview review, critique, and Golden Answer (Interview Critique)

#### C1. Input and preprocessing

- **Input**: transcript (question–answer pairs), resume, JD
- **Structuring**: split the transcript into QA units; annotate timestamps if available
- **Retrieved context**
  - retrieve project/technical-detail chunks relevant to the question from the private resume store
  - optionally retrieve frequent points for similar questions from the public `company_intel` store
  - extract key role capabilities from the JD for scoring-dimension weights

#### C2. Intent analysis → scoring → critique → optimization (recommend four subchains)

- **IntentChain**: identify what the interviewer is assessing (depth/breadth/communication/trade-offs/boundary conditions)
- **ScoreChain**: output 0–100 per dimension (logic/completeness/technical accuracy/clarity/JD fit)
- **CritiqueChain**: identify gaps (missing facts/jumps in reasoning/unclear concepts/inconsistent metrics)
- **GoldenAnswerChain**: generate optimized wording based on evidence-backed resume facts (STAR), and provide follow-up branches for preparation

#### C3. Report artifacts (structured by default)

- Per question: intent, score, issues, improvement suggestions, Golden Answer, evidence references (`chunk_id` / `fact_id`)
- Overall: strengths/weakness radar and next-round training plan (recommended question sets and review checklist)

---

## Phase 4: LangChainGo Logic Integration Standards (maintainable AI orchestration, 5–10 days)

### 4.1 Unified wrapper layer (suggested location: `pkg/ai/`)

- **ModelClient**
  - uniformly wrap multi-model calls (OpenAI/Claude/local), with timeout, retry, circuit breaking, and fallback
  - output must support “structured JSON + parsing validation”
- **EmbeddingClient**
  - support multiple embedding-model versions; record `embed_model/embed_dim` when writing `im_chunk_embedding`
- **Retriever**
  - `VectorRetriever`: OceanBase vector retrieval + filter conditions (tenant/user/doc_type/company/role)
  - `HybridRetriever`: semantic + keyword (later enhancement)
- **PromptRegistry**
  - prompt versioning: `prompt_key + version`; changes must be traceable and rollbackable

### 4.2 Chain design (split into small units for testability and composition)

- **ResumeExtractChain**: resume → JSON (facts)
- **ResumeRiskWeightChain**: facts → risk points and weights
- **QuestionGenChain**: facts + role/jd + retrieved chunks → question-set JSON
- **InterviewIntentChain / ScoreChain / CritiqueChain / GoldenAnswerChain**

> Recommendation: define input/output structs for every Chain and provide offline replay tests (given fixed input and fixed retrieval results, the output can be snapshotted).

### 4.3 Tools and Memory (for multi-turn dialogue and controlled calls)

- **Tools (suggested)**
  - `SearchCompanyIntelTool`: call `search-svc` for the latest clustering summary
  - `RetrieveResumeChunksTool`: retrieve from the private resume store by query
  - `RetrieveInterviewContextTool`: retrieve historical QA and report versions by `session_id`
- **Memory (suggested)**
  - **Short-term memory**: the current session’s recent N QA pairs and preferences
  - **Long-term memory**: user preferences, common weakness tags, and learning plans (stored in `im_user_setting`/a dedicated table)

---

## Phase 5: Asynq Async Task Flow (stable throughput and retryability, 5–10 days)

### 5.1 Typical time-consuming task decomposition

#### Task 1: Resume parsing and storage (CPU/IO)

- **Job**: `resume.parse`
- **Steps**: download file → extract text → clean → extract facts → write doc/chunk → enqueue embedding
- **Idempotency**: use `resume_id + version + file_hash` as the idempotency key; repeated submissions reuse the result

#### Task 2: Embedding generation (external call/batch)

- **Job**: `embedding.generate`
- **Steps**: load chunks by doc_id → batch embedding → write `im_chunk_embedding`
- **Strategy**: batch, rate limit, retry failures (exponential backoff), and record failed chunks for compensation

#### Task 3: Interview-experience crawling and cleaning (external network)

- **Job**: `company.ingest`
- **Steps**: search links → crawl body → remove noise → chunk → embedding → clustering summary (may be split)

#### Task 4: Audio-to-text (ASR)

- **Job**: `asr.transcribe`
- **Steps**: fetch audio → call ASR → generate transcript → write doc/chunk → enqueue critique

#### Task 5: Deep critique and review (long LLM task)

- **Job**: `interview.critique`
- **Steps**: load QA by session → execute subchains (intent/score/critique/golden) concurrently for each question → aggregate → write report
- **Concurrency control**: rate-limit by `user_id` to prevent a single user from exhausting the quota

### 5.2 Queue and priority suggestions

- `q.realtime`: user-triggered tasks requiring a fast response (light generation/small-batch embedding)
- `q.batch`: interview-experience crawling and batch embedding
- `q.heavy`: deep critique and long-text summarization

### 5.3 Unified task-status model (suggested)

- `im_task`: `task_id`, `type`, `ref_id` (resume_id/session_id/doc_id), `status` (pending/running/succeeded/failed), `progress`, `error`, `created_at`, `updated_at`
- The API returns `task_id`; the frontend polls or receives updates through WebSocket/SSE (full release).

---

## Phase 6: Staged Milestones (Roadmap: MVP → Beta → Full Release)

### 6.1 MVP (corresponds to Requirements Phase 1: resume parsing + question prediction, 2–3 weeks)

- **Must deliver**
  - resume upload/parsing/structured facts
  - resume-based question prediction (3–5 questions per project + assessment points + follow-up branches)
  - OceanBase: metadata + chunks + embeddings (at least a private resume vector store)
  - Asynq: `resume.parse` + `embedding.generate`
- **Acceptance metrics**
  - parsing success rate, question readability, and evidence consistency (no fabrication)

### 6.2 Beta (corresponds to Requirements Phase 2: interview-experience retrieval + RAG fusion, 2–4 weeks)

- **Must deliver**
  - company+role interview-experience crawling/storage/vector retrieval
  - frequent-question clustering and interview-style summary
  - resume question prediction can fuse “target-company style” context
- **Acceptance metrics**
  - interview-experience update availability (success rate/duplication rate/cleaning quality)
  - retrieval hit rate and summary consistency

### 6.3 Full release (corresponds to Requirements Phase 3: ASR + review/critique loop, 3–6 weeks)

- **Must deliver**
  - audio upload → ASR → structured transcript
  - per-question intent analysis, scoring, critique, and Golden Answer (with evidence references)
  - review report and training plan (weakness tags + recommended question sets)
  - resource and cost governance (rate limiting, circuit breaking, quotas, caching, fallback)
- **Acceptance metrics**
  - end-to-end latency (return “task accepted + preliminary result” within 5–10 seconds; complete the deep report asynchronously)
  - report consistency and traceability (references to `chunk_id`/`fact_id`)

---

## Key Engineering Recommendations (across all phases)

### 1) Prefer structured output

All AI artifacts should preferably be JSON (scores, intent, issues, Golden Answer, evidence references); the business layer only renders and archives them.

### 2) Treat “do not fabricate” as a system-level constraint

Resume/interview generation must include evidence references; when evidence is insufficient, explicitly output uncertainty and the information that needs to be supplemented.

### 3) Versioning and replayability

- Prompt, model, embedding, and chunking strategies must be recordable.
- Key pipelines must support offline replay (same input, same retrieval results) for regression testing.

### 4) Prevent cross-tenant access in retrieval

The retrieval layer must always append `(tenant_id, owner_type, owner_id)` filters to prevent vector recall from leaking across users.
