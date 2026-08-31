# InterviewMaster Agent Implementation TODO

> **Archived (2026-08-28).** Do not use this file for scheduling. Current checklist: [`../invite-beta-todo_0828.md`](../invite-beta-todo_0828.md).
> Generated: 2026-08-12
> Basis: [`launch-todo.md`](launch-todo.md), [`../agent-design.md`](../agent-design.md), and a review against the code as it existed at that time
> Goal: upgrade the existing rule-based Demo into a real AI interview system that supports gradual rollout, auditing, cost control and regression validation

---

## 0. Scope and definition of done

This checklist covers only the intelligence layer and its direct dependencies: model Providers, Prompts, resume understanding, question generation, dynamic follow-ups, scoring, reporting, auditing, cost, evaluation and related frontend/backend state.

Not implemented in this round:

- Autonomous multi-Agent collaboration.
- Generic long-term memory or context compression.
- MVP private-resume RAG.
- MCP tool ecosystem.
- Large-scale public interview-content vector database.

The initial architecture remains:

```text
Deterministic workflow (resume extraction → blueprint → questions → scoring → report)
                                      +
One narrowed dynamic follow-up decision (the business state machine controls the exit)
```

### Status markers

- `[ ]` Not started.
- `[-]` Skeleton exists or in progress, but acceptance has not passed.
- `[x]` Completed and passed the corresponding acceptance.
- `[!]` Blocked by an external condition or product decision.

### Agent MVP definition of done

The intelligence layer is considered ready for an invite-only launch only when all of the following are true:

1. Questions produce explainable differences for different resumes, JDs and target roles, instead of always returning five fixed questions.
2. Reports score based on questions, answers and material evidence, instead of answer length.
3. All model output first enters strongly typed structures and is validated; invalid output never goes directly into business tables.
4. Every call is traceable to its Provider, model, Prompt version, tokens, latency, cost and Trace.
5. When the model times out, is rate-limited or unavailable, or returns invalid output, the user sees an explicit status and can safely retry or receive controlled degradation.
6. Every user has concurrency, call-count and cost limits; unbounded calls cannot occur.
7. The Golden Dataset and the real full-flow E2E meet the release baseline.

---

## 1. Confirmed design decisions

The following decisions constrain this implementation round and should not be repeatedly revisited during coding:

| Question | Decision |
| --- | --- |
| Architecture shape | Do not build autonomous multi-Agent systems; use a deterministic workflow with bounded dynamic follow-ups |
| Orchestration framework | Eino may be used internally in `platform/ai`; domain logic must not import Eino directly |
| Business dependencies | The domain layer depends only on self-defined `ChatModel`, `EmbeddingModel`, `Tool` and other interfaces |
| MVP RAG | Do not build it; feed the resume/JD directly after length control, with evidence relying on fact IDs and original-text locations |
| Embedding | MVP does not depend on it; remove or stop adding SHA-256 fake vectors and do not present them as a real capability |
| State and memory | PostgreSQL is the authoritative state; assemble recent context by rule at call time and do not use independent Agent Memory |
| Tenant identifier | The current product isolates by `user_id`; do not mix in a fictional `workspace_id` before a workspace table exists |
| Scoring principle | The model outputs evidence and judgments for each dimension; Go calculates the total score using fixed rules |
| Model-call location | Long tasks run in the Worker; only follow-ups that need interactive latency may call synchronously |
| Failure principle | Failures are visible; retry only clearly retryable errors, and never fabricate AI results |

---

## 2. Current baseline

### Existing foundation

- [x] API, Worker, PostgreSQL, Redis, object storage and asynchronous-task foundations.
- [x] Foundations for resume upload, Tika text extraction, JD, question sets, the interview state machine and report data tables.
- [x] JWT + Refresh Session, user-level resource isolation and login rate limiting.
- [x] OpenTelemetry Trace skeleton and structured HTTP logs.
- [x] Real adapters for Embedding/Tool: OpenAI embeddings, read-only tools, a pgvector Retriever and MCP list/call adapters. MVP private resumes still primarily use full-text input.
- [x] `backend/prompts/` contains executable Prompts and Schemas for resume extraction, blueprints, questions, scoring and reports.

### Stubs that must be replaced

- [x] Formal question generation uses a blueprint + asynchronous Worker; when AI is disabled, it writes `degraded` rule questions rather than treating them as a formal success.
- [x] Reports now use dimension scoring and asynchronous generation; character-count scoring has been removed.
- [x] Resume parsing retains Tika text and, when AI is enabled, calls `resume.extract/v1` to extract facts with original-text excerpts.
- [x] New chunks no longer write SHA-256 fake vectors; `embedding` remains NULL.
- [x] Interview-content retrieval now uses keyword retrieval over local `public_intel_items` corpus; the frontend still has no formal entry point.

---

## 3. P0-A: model infrastructure and configuration

### AGENT-001 Formalize self-defined interfaces

- [x] Keep `ChatModel.Generate`, and complete request-level deadlines, structured-output constraints and call metadata.
- [x] The initial release has no streaming UI; do not expand the `Stream` interface yet.
- [x] Unify tenant fields as the currently real `UserID`; support `WorkspaceID` only after workspaces are introduced.
- [x] Define unified error types: timeout, rate limit, authentication, provider error, invalid output, context overflow and budget exhaustion.
- [x] Keep the Provider's original error in the error cause; do not use it directly as client-facing copy.
- [x] Provide a Fake Model implementation so Eino Graph business tests do not depend on a real external model.

Acceptance:

- Business packages do not import Eino or Provider SDKs.
- Providers can be replaced by a Fake implementation, and core workflow unit tests need no network.

### AGENT-002 Integrate one real Provider

- [x] Pin Eino `v0.9.13` and the OpenAI component `v0.1.13`; do not use `@latest`.
- [x] Implement an OpenAI-compatible ChatModel adapter under `backend/internal/platform/ai/provider/`.
- [x] Support Base URL, API Key, model name, request timeout and maximum output-token environment settings.
- [x] Inject the same audit/quota wrapper through `airuntime.NewChatModel` in both API and Worker.
- [x] At startup, validate Provider, key, model, timeout and token limits when AI is enabled.
- [x] Map connection errors, 429, 5xx and timeout errors for the Provider.

Suggested environment variables:

```text
IM_AI_PROVIDER=
IM_AI_BASE_URL=
IM_AI_API_KEY=
IM_AI_CHAT_MODEL=
IM_AI_SMALL_MODEL=
IM_AI_REQUEST_TIMEOUT=
IM_AI_MAX_OUTPUT_TOKENS=
```

Acceptance:

- A minimal request can obtain a real model response locally.
- Without a key, AI can be explicitly disabled in development; production startup fails.
- The API Key does not enter logs, database input snapshots or client responses.

### AGENT-003 Call reliability strategy

- [x] HTTP Client and whole-call deadlines are set; first-byte timeout will be added when streaming calls are introduced.
- [x] Exponential backoff with jitter is used only for transient network errors, 429 and recoverable 5xx; retry at most twice.
- [x] 4xx parameter errors, output-validation errors and budget rejections do not use transport retries.
- [x] JSON repair is attempted at most once and is counted separately from transport retries.
- [x] Transport retries reuse the same business request; audit records the final attempt result. Per-attempt details can be added later.
- [x] When a Provider is disabled or unavailable, question-set/report generation enters `degraded` or `failed`; fallback output follows the same domain structure.

---

## 4. P0-B: audit, cost and quota first

### AGENT-010 Add `model_invocations`

- [x] Add a database migration and queries recording at least:

```text
id
user_id
task_id / session_id / resource_type / resource_id
provider / model
prompt_key / prompt_version
status / attempt
input_hash / output_hash
prompt_tokens / completion_tokens / total_tokens
estimated_cost
latency_ms
error_code
trace_id / request_id
created_at / completed_at
```

- [x] By default, do not store complete resumes, JDs, answers or model-output bodies; store only input/output hashes.
- [x] Add query indexes for `user_id + created_at`, `resource_id` and `trace_id`.
- [x] Use a unified `InstrumentedChatModel` for instrumentation; business nodes must not write audits independently.
- [x] Audit-write failures are logged; production blocks, while development degrades and continues.

### AGENT-011 Cost and quota

- [x] Set maximum input characters/tokens, maximum output tokens and maximum duration per call.
- [x] Set a per-user limit on simultaneous model calls.
- [x] Set per-user daily call limits and daily/monthly soft and hard budgets.
- [x] The soft threshold selects a small model; the hard threshold returns `AI_BUDGET_EXHAUSTED`.
- [x] Reuse in-progress/completed generation for question sets by input hash; bind reports uniquely to sessions.
- [x] Administrators can inspect costs and abnormal users using aggregate SQL documented in comments on the `model_invocations` table.

Acceptance:

- Any business generation can be traced to token usage, latency, cost and result status.
- Concurrency load cannot bypass the user's hard quota.
- Repeated requests do not create unbounded duplicate charges or duplicate business results.

---

## 5. P0-C: Prompts and strongly typed output

### AGENT-020 Build an executable Prompt package

- [x] Six Prompt packages have been created and embedded into the binary:

```text
backend/prompts/resume.extract/v1/
backend/prompts/blueprint.generate/v1/
backend/prompts/question.generate/v1/
backend/prompts/evaluation.critique/v1/
backend/prompts/interview.followup/v1/
```

- [x] Each directory contains `system.md`, `task.md`, `meta.yaml` and an output Schema.
- [x] The Prompt loader uses `embed.FS`; container runtime does not depend on the working directory.
- [x] Prompts are stored in versioned directories; upgrades add a new version directory instead of overwriting the old one in place.
- [x] Every Prompt separates System, task, Schema and untrusted data.
- [x] Inputs are rune-trimmed by field while preserving untrusted-data boundaries.

### AGENT-021 Define domain output contracts

- [x] Define `ResumeExtraction`: fact category, fact value, confidence, `source_excerpt` and original-text location.
- [x] Define `InterviewBlueprint`: target capabilities, weights, difficulty, number of main questions, follow-up budget and time budget.
- [x] Define `GeneratedQuestionSet`: questions, intent, expected points, capability key, difficulty, evidence fact IDs and follow-up suggestions.
- [x] Define `TurnEvaluation`: scoring dimensions, scoring reasons, evidence, missing information and improvement suggestions.
- [x] Define `InterviewReportDraft`: strengths, improvement items and training steps; the model may not overwrite the deterministic total score.
- [x] Provide a Go Struct, business validator and JSON Schema for each structure.
- [x] Validate enums, counts, lengths, score ranges, ID references, duplicates and evidence ownership in Go.

### AGENT-022 Unify structured-output execution

- [x] StructuredRunner performs the model call, local JSON Schema validation, strict JSON decoding and unknown-field rejection.
- [x] Native Provider JSON Schema and domain validation are connected, with a local general-purpose JSON Schema validator added.
- [x] On the first failure, send a concise validation error to a repair Prompt, at most once.
- [x] If the second attempt still fails, return `AI_OUTPUT_INVALID` and do not write partial business data.
- [x] Save failure type and output hash for model/Prompt regression analysis.

---

## 6. P0-D: real resume understanding

### AGENT-030 Upgrade resume extraction to an asynchronous AI node

- [x] Keep Tika responsible for file-to-text conversion; the LLM processes only extracted text.
- [x] Establish preprocessing rules for text length, whitespace, garbling and overly long resumes.
- [x] Call `resume.extract/v1` to generate structured facts for skills, experience, projects, metrics and education.
- [x] Every fact must include a verifiable original-text excerpt and the owning resume version.
- [x] Replace the old facts for that version in one transaction to avoid mixing extraction versions.
- [x] Save `extractor_model`, `prompt_version` or an associated invocation ID to support re-parsing and traceability.
- [x] On extraction failure, retain Tika text and a retryable task state; do not incorrectly mark the resume as completed.

### AGENT-031 Remove fake Embedding semantics

- [x] MVP no longer writes SHA-256 pseudo-vectors for new chunks.
- [x] Retain old `baseline-hash-1536` data for compatibility, but do not use it for new writes.
- [x] If chunks are retained temporarily, store only text slices and `token_count`; do not claim semantic retrieval capability.
- [x] The Embedding field and pgvector extension may remain as a future compatibility interface.

Acceptance:

- The same resume version produces stable facts on repeated runs, or differences are attributable to the model/version.
- Every fact can be traced to the source text and resume version.
- No new pseudo-vector is written.

---

## 7. P0-E: real question-set generation

### AGENT-040 Blueprint

- [x] Decide blueprint ownership: persist it on the question set after generation and copy a snapshot when creating an interview.
- [x] Add a `blueprint` JSONB field and record Schema/Prompt/model versions.
- [x] The blueprint contains at least capability keys, weights, difficulty, question counts, follow-up limits and evidence scope.
- [x] Historical interviews always read the creation-time snapshot and are not overwritten by later regeneration.

### AGENT-041 Question-set generation workflow

- [x] Input the current user's resume text, structured facts, optional JD and target role.
- [x] Generate the blueprint.
- [x] The Worker generates the blueprint before generating questions.
- [x] Validate counts, continuous ordinals, field lengths, unknown fields, capability coverage and evidence IDs.
- [x] For invalid output, perform one targeted repair instead of regenerating all content that already passed.
- [x] Model calls have moved outside the database write transaction; write only after successful validation.
- [x] Use input resource version + Prompt version as the idempotency key.
- [x] The formal AI path no longer calls fixed questions; when AI is disabled, local fallback is marked `degraded`.

### AGENT-042 Switch to an asynchronous product flow

- [x] Question-set creation returns `202 + task_id`.
- [x] The Worker performs blueprint and question generation and updates progress stages.
- [x] The frontend displays real stages such as “read materials, plan capabilities, generate questions, quality check.”
- [x] Support failure retry and reuse existing question sets by input hash to avoid duplicate charges.
- [x] The frontend must not wait for complete generation through a 60-second HTTP request.

Acceptance:

- Results generated from different resumes/JDs have material relevance and differ appropriately.
- Every material-related question is traceable to a fact ID or explicitly marked as role-generic.
- Duplicate output, unauthorized evidence IDs, invalid JSON and model timeouts have automated tests.

---

## 8. P0-F: real scoring and reporting

### AGENT-050 Establish the scoring rubric

- [x] Define five scoring dimensions, weights and 0–100 anchor definitions.
- [x] Distinguish “no evidence provided” from “insufficient capability,” so a short but valid answer is not penalized merely for length.
- [x] Define scoring rules for skipped questions, empty answers and unavailable material.
- [x] Have Go validate each dimension and deterministically aggregate the total score.
- [x] Reports must state that scores are training advice, not hiring conclusions.

### AGENT-051 Scoring workflow

- [x] Intent: read question intent, capability points and expected answer points.
- [x] Evidence: extract supporting/contradicting evidence from the answer, resume facts and context.
- [x] Evaluation: output scores and reasons for each dimension using the fixed rubric.
- [x] Critique: generate specific, actionable improvement advice without fabricating experience.
- [x] Evaluate questions in order and limit model calls with the user-level concurrency quota.
- [x] If one question fails, fail the whole report and allow retry; never silently fill in a fixed high score.

### AGENT-052 Report generation

- [x] Go aggregates per-question dimension scores, total score and quality gates.
- [x] The report model only organizes strengths, improvement priorities and a training plan; it must not change scores.
- [x] Golden Answers must be based on the user's existing materials; assumptions are explicitly labeled as example wording.
- [x] Evidence returns stable references such as fact IDs/answer excerpts, rather than generic text such as “the user provided an answer.”
- [x] Reports are uniquely bound to sessions; repeated queries reuse results.
- [x] Remove the `scoreAnswer()` character-count score and fixed critique/Golden Answer.

### AGENT-053 Make report generation asynchronous

- [x] Enqueue the report task when an interview is completed; GET report only reads status/results and does not call the model inside the request transaction.
- [x] Support `pending/running/completed/failed/degraded` states.
- [x] The frontend polls or subscribes to status and provides a safe retry entry point.
- [x] A report-generation failure must not affect the completed interview or user answers.

Acceptance:

- Score variance for repeated evaluation of the same answer remains within the defined range.
- More specific, evidence-backed answers generally score better than vague or irrelevant answers.
- Facts, numbers and technologies in the report are traceable to materials or answers.
- Empty answers, Prompt Injection, overlong answers and mixed Chinese/English responses have tests.

---

## 9. P1: dynamic interviewer and narrowed ReAct

Implement this stage only after real question generation and scoring are stable, to avoid introducing too much uncertainty into the initial release.

### AGENT-060 Start with a restricted decision-maker

- [x] The first version prioritizes strongly typed decision output instead of immediately opening arbitrary Tool Calling:

```json
{
  "action": "follow_up",
  "question": "...",
  "capability_key": "...",
  "evidence_fact_ids": ["..."],
  "reason": "..."
}
```

- [x] `action` permits only `follow_up` and `next_capability`.
- [x] The Go state machine decides whether to accept the action, whether to end the interview and the next ordinal.
- [x] At most one follow-up decision is made per round; on timeout, use the blueprint's static next question.

### AGENT-061 Complete the dynamic-session model

- [x] Save the blueprint snapshot, current capability, used follow-up count and model version on the session.
- [x] Add main-question/follow-up type, `parent_turn_id`, capability key and generation invocation ID to turns.
- [x] Clarify whether dynamic follow-ups consume total-question and total-time budgets.
- [x] Each round's context contains only system rules, the current blueprint point, the latest 2–3 rounds and the current answer.
- [x] Recover from the database after refresh or Worker/API restart; do not depend on in-process message history.

### AGENT-062 Upgrade to Tool/ReAct only when needed

- [x] Use ADK ChatModelAgent only after real “look up resume excerpts” or “look up interview content” tools provide value.
- [x] Tools must be read-only and internally bind the current `user_id`; model parameters cannot override tenant scope.
- [x] Tool parameters use JSON Schema, and returned row/character counts are limited.
- [x] Each round allows at most two tool calls; loop depth, tokens and time all have hard limits.
- [x] On tool timeout/empty result, return to the blueprint's static next question.
- [x] Audit tool calls, result hashes and rejection reasons.

Acceptance:

- The model cannot add unlimited rounds or end the entire interview by itself.
- A malicious answer cannot induce a tool to read another user's data.
- Follow-up timeout, Provider failure and tool failure all fall back and complete the full interview flow.

---

## 10. P1: security, privacy and Prompt Injection

### AGENT-070 Prompt Injection defense

- [x] Make system-instruction priority explicit; resumes, JDs, interview content and answers are all untrusted data.
- [x] User content may not change the output Schema, tenant scope, tool list, budget or system role.
- [x] The tool service performs authorization again; do not trust user or resource IDs supplied by the model.
- [x] Build an evaluation set for attacks such as “ignore the rules,” “leak the Prompt” and “read another resume.”
- [x] Logs and error responses must not disclose System Prompts, API Keys or complete user materials.

### AGENT-071 Data governance

- [x] Clarify the model Provider's data-retention and training policies; production uses interfaces/accounts that do not train on the data.
- [x] Disclose in the privacy policy that resumes, JDs and answers are sent to model Providers for processing.
- [x] Support account-data export/deletion and a compliant retention policy for model-audit records.
- [x] Set retention periods for model input, audit-debug bodies and object storage.
- [x] Redact emails, answer bodies, resume excerpts and Tokens in logs.

---

## 11. P1: evaluation, testing and release gates

### AGENT-080 Golden Dataset

- [x] Build a versioned, anonymized sample set covering a Chinese new graduate, an English experienced hire, empty answers, off-topic answers and Prompt Injection.
- [-] A large manually labeled set for fact extraction/question relevance/human-score comparison still needs to be accumulated.
- [x] Fake Model regression covers empty-answer score 0, injection not changing the action and unknown-field rejection.
- [x] Add empty-answer, off-topic and Prompt Injection samples.
- [x] Prompt/Schema changes can regress through `go test ./internal/aieval ./internal/aiworkflow ./internal/platform/ai/...`.

### AGENT-081 Automated tests

- [x] Provider adapter unit tests: normal response, 429, 5xx, timeout, invalid JSON and missing Token usage.
- [x] Prompt loading and missing-version tests.
- [x] Unit tests for every domain Schema and validator.
- [x] Fake Model-driven unit tests for resume, question-generation, scoring, report and follow-up workflows.
- [-] PostgreSQL + Redis + Worker integration tests: duplicate delivery, crash, retry, idempotency and dead-letter handling.
- [x] Cross-user access, unauthorized evidence IDs and tool-authorization security tests.
- [-] Full Playwright E2E: register → upload → parse → JD → question set → interview → report.
- [-] Low-volume real-Provider smoke test with production-equivalent configuration; the key comes only from CI Secret.

### AGENT-082 Quality gates

- [x] Define fact-extraction precision/recall baselines.
- [x] Define question material-relevance, duplicate-rate and capability-coverage baselines.
- [x] Define scoring stability and correlation with human scores.
- [x] Define invalid-structured-output rate, model-error rate, P95 latency and per-call cost limits.
- [-] Block Prompt or model versions from launch when any hard baseline is missed.
- [x] Support fast rollback to the previous Prompt/model combination.

---

## 12. P1: observability and operational protection

### AGENT-090 Metrics

- [x] Model request count, success rate, retry rate and degradation rate.
- [x] Time to first token/total latency; if there is no streaming, at least record total latency.
- [x] Input, output and total Tokens, plus estimated cost.
- [x] First structured-output failure rate, repair success rate and final failure rate.
- [x] Duration and failure rate for each workflow node.
- [x] Queue wait time, execution time and backlog for question-set/report tasks.
- [x] Abnormal per-user call volume and budget-rejection count.

### AGENT-091 Alerts and Runbook

- [x] Provider error rate or P95 latency above threshold.
- [x] Significant increase in invalid-output rate.
- [x] Queue backlog or tasks running for too long.
- [x] Abnormal per-user/site-wide Token or cost usage.
- [x] Audit-write failure and missing cost-price table.
- [x] Write investigation, degradation, disablement and recovery steps for each alert type.

---

## 13. P2: future capabilities

- [x] Integrate a real retrieval Provider after the interview-content data source passes compliance review.
- [x] Use real Embedding after the public interview-content corpus reaches retrieval scale.
- [x] Build a self-owned pgvector Retriever; all private queries must be filtered by `user_id`/future workspace scope.
- [x] Keep the public interview-content library and users' private materials in separate data domains with separate authorization policies.
- [-] Build retrieval-recall, empty-result and citation-correctness evaluation.
- [x] Implement an MCP Tool Adapter only when an external tool ecosystem is needed.
- [x] Add a structured `user_skill_profile` after multiple interviews, but allow users to view, correct and delete it.
- [x] Consider streaming output, real-time voice interviews or more complex Agent collaboration only after clear product value is established.

---

## 14. Recommended implementation batches

### Batch 1: connect the minimum real AI path

- AGENT-001–003: interfaces, Provider and reliability.
- AGENT-010: call auditing.
- AGENT-020–022: Prompt and structured-output runner.
- AGENT-030: real resume fact extraction.

Deliverable: after uploading one resume, obtain real structured facts with original-text evidence.

### Batch 2: deliver real core user value

- AGENT-040–042: asynchronous blueprint and question-set generation.
- AGENT-050–053: asynchronous scoring and report generation.
- Connect frontend task status, question-set and report pages at the same time.

Deliverable: different materials produce different questions, and a completed interview produces a real evidence-backed report.

### Batch 3: establish launch gates

- AGENT-011: quotas and cost.
- AGENT-070–071: security and data governance.
- AGENT-080–082: Golden Dataset, tests and gates.
- AGENT-090–091: monitoring, alerts and Runbook.

Deliverable: observable, quota-limited, rollback-capable and ready to pass an invite-only Beta launch check.

### Batch 4: enhance interaction

- AGENT-060–062: dynamic follow-ups and on-demand ReAct.
- Implement future P2 capabilities such as interview-content retrieval, Embedding and MCP according to actual data scale.

Deliverable: a bounded, recoverable and degradation-safe dynamic interviewer.

---

## 15. Launch checklist

### Must all pass before invite-only Beta

- [x] Fixed five questions, character-count scoring and fixed reports have been removed from the formal path.
- [x] Provider and Prompt versions are fixed and can roll back to the previous version directory/environment setting.
- [x] Question sets and reports use asynchronous tasks; interfaces do not hold a transaction while waiting for a model.
- [x] All model output passes Schema and domain validation.
- [x] Model-call audit, Tokens, cost and Trace can be queried.
- [x] User concurrency, daily-call and hard-budget limits are active.
- [x] Prompt Injection, cross-user evidence access and log-leakage tests pass.
- [-] Core E2E, Worker integration tests and the Golden Dataset meet the baseline.
- [x] When the model is unavailable, question sets/reports show an explicit failure or degradation state and can be safely retried.

- [x] The privacy policy and AI-scoring statement cover third-party model processing.

### Additional requirements before public registration

- [-] Capacity and cost load testing is complete.
- [x] Monitoring, alerts and Runbook are connected; a full incident drill is still pending.
- [x] Data export, account deletion and data-retention policies are live.
- [ ] Provider quota exhaustion, regional outage and key-rotation drills pass.
- [ ] Release, rollback and database backup/recovery procedures have been rehearsed.

---

## 16. First-round development recommendation

Do not implement Eino Graph, Workflow, ADK, RAG and MCP all at once. First establish one minimal vertical slice:

```text
Real Provider
  → model_invocations
  → question.generate/v1
  → Go Struct + Schema validation
  → Fake Model unit tests
  → Generate a real question set from one resume/JD
```

This path validates six key boundaries: configuration, model calls, Prompt loading, structured output, auditing and business persistence. Once stable, reuse the same foundation to extend resume extraction and scoring/reporting.
