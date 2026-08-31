# InterviewMaster Agent Architecture and API Refactor TODO

> Date: 2026-08-14
> Design basis: `agent-design.md`
> Baseline when written: database v9, existing Interview API, `question:generate` / `report:generate` Worker
> **Archived; this is only a historical implementation record and is not the current task list.** Status on 2026-08-28: the V2 body has landed (schema v10, `interview:prepare` / `interview:next_turn`, interview-room UI). The current invite-only execution checklist is [`../invite-beta-todo_0828.md`](../invite-beta-todo_0828.md).
> Goal: change “fixed candidate questions + an AI follow-up plug-in” into a “blueprint state machine + real-time generation of the next question each round”

---

## 0. Document positioning

This document describes the Agent V2 architecture, interfaces, data migration and implementation.

- `agent-design.md` defines the target principles and product behavior.
- This document defines how interfaces evolve and how work is broken down; its implementation checkboxes were already outdated when it was written.
- [`agent_todo.md`](agent_todo.md) is an earlier infrastructure checklist and is no longer the acceptance basis for the main loop.
- The current HTTP contract is defined by `backend/api/interviewmaster.api`.

### Status markers

- `[ ]` Not started.
- `[-]` Started but acceptance not completed.
- `[x]` Completed and passed the corresponding acceptance.
- `[!]` Blocked by an explicit external dependency.

### Not in this round

- Autonomous multi-Agent systems.
- Generic long-term memory.
- Private-resume vector RAG.
- Real-time voice conversation, TTS or full-duplex WebSocket.
- New frontend, client or Agent-development roles.
- Letting users manage internal question sets, blueprints or asynchronous tasks.

---

## 1. Current implementation baseline and main problems

### 1.1 Foundation that can be retained

- Self-defined `ChatModel` / `Tool` abstractions and Eino Provider adapter.
- Prompt versioning, JSON Schema, strict decoding and one repair attempt.
- `model_invocations` audit, Tokens, cost, rate limiting and retries.
- PostgreSQL session state, resume-version snapshots and complete Turn records.
- Asynq Worker, task claiming, idempotency and failure-compensation foundations.
- Resume-fact tools, company-information tools and assembly of the latest three rounds of context.
- Interview records, per-turn reports and paginated pages.

### 1.2 Behavior that must be replaced

| Current behavior | Target behavior |
| --- | --- |
| Blueprint and Prompt always produce five fixed questions | Blueprint stores minimum, target and maximum rounds plus a capability-coverage strategy |
| `questions` is the playback queue for a normal interview | `questions` stores only anchors, Rubrics and degradation fallbacks |
| Only `follow_up` generates questions in real time | Both deepening and switching capabilities must generate the next question in real time |
| `next_capability` takes a question from the candidate list | The Agent returns a capability key and a complete, specific question |
| The model can directly `finish` | The model can only `recommend_finish`; the state machine decides whether to end |
| The answer endpoint waits for the model synchronously | Persist the answer first, enter recoverable `deciding`, and generate the next question asynchronously |
| Recovery after an interruption relies on guessing from a 15-second timeout | Recover with a persisted decision and unique key |
| Dynamic Turns lack scoring fields | Save a complete question snapshot for every real Turn |
| Resume tools search every version belonging to the user | Tools are bound to the current `resume_version_id` |
| When AI is disabled, the product silently looks like a normal interview | Explicitly return and display `rule` demo mode |
| Generic `/tasks/:id` is exposed to the business frontend | Replace it with interview-domain preparation, decision and report-retry interfaces |

---

## 2. Target architecture

### 2.1 Component responsibilities

~~~text
Web
  ├─ Create an interview / view preparation status
  ├─ Submit an answer / display deciding
  ├─ Poll the current Session (MVP)
  └─ View interview records and reports

API
  ├─ Authentication and tenant isolation
  ├─ Create Sessions and an internal blueprint carrier
  ├─ Atomically save answers, phase and decisions
  ├─ Query aggregated interview state
  ├─ Let users end an interview
  └─ Business-specific retry entry points

Worker
  ├─ interview:prepare
  ├─ interview:next_turn
  └─ report:generate

AI Workflow
  ├─ ResumeExtraction
  ├─ BlueprintV2
  ├─ InterviewMaterials
  ├─ NextTurnDecisionV2
  ├─ TurnEvaluationV2
  └─ ReportCompose

PostgreSQL
  ├─ Session policy and phase
  ├─ Capability Progress
  ├─ Immutable Turn question snapshots
  ├─ Next-turn Decision idempotency state
  └─ Model Invocation audit
~~~

### 2.2 Main flow

~~~text
POST /interviews
  → Create a preparing Session
  → Create an interview:prepare task
  → Worker generates BlueprintV2, internal materials and the first question
  → Session = active / answering

PUT /interviews/:id/turns/:ordinal/answer
  → Atomically save the answer
  → Session.phase = deciding
  → Create a unique next-turn decision
  → Enqueue interview:next_turn
  → Return 202 immediately

Next-turn Worker
  → Read the blueprint, capability progress, current answer and latest rounds
  → Make at most two read-only tool calls
  → Output NextTurnDecisionV2
  → Let the server policy decide whether to end or generate the next question
  → Atomically update progress, Turn and phase

GET /interviews/:id
  → Short-poll from the frontend
  → Display the next question once phase becomes answering
~~~

### 2.3 Layer boundaries

- `backend/internal/platform/ai`: Providers, Eino, ReAct, structured output, auditing and tool adapters.
- `backend/internal/aiworkflow`: strongly typed Blueprint, Materials, NextTurn, Evaluation, Report and other nodes.
- `backend/apps/api/internal/logic/workspace`: business transactions, state machine, API errors and task enqueueing.
- `backend/apps/worker/internal/tasks`: recoverable asynchronous execution; it does not assemble HTTP responses.
- Prompts do not access the database; the server binds tool calls to the tenant and resume version.

---

## 3. State-model adjustments

### 3.1 Public Session states

Continue using:

~~~text
preparing
active
completed
failed
abandoned
~~~

Semantics:

- `preparing`: the blueprint, internal materials or first question is not complete.
- `active`: the interview can continue; the specific stage is represented by `phase`.
- `completed`: interview Q&A has ended; the report may still be generating.
- `failed`: used only for an unrecoverable preparation-stage failure.
- `abandoned`: the user explicitly abandoned the interview without requesting a complete report.

### 3.2 Internal phases under Active

Add `phase`:

~~~text
preparing
answering
deciding
decision_failed
completed
~~~

Constraints:

- When `status=active`, phase may only be `answering`, `deciding` or `decision_failed`.
- `phase=answering` must have exactly one unanswered current Turn.
- `phase=deciding` must have one answered Turn and its corresponding pending/running decision.
- `phase=decision_failed` must retain the answer, error summary and retryable decision.
- When `status=completed`, phase is fixed at `completed`.

### 3.3 State transitions

~~~text
preparing/preparing
  ├─ Preparation succeeds → active/answering
  └─ Preparation fails → failed/preparing

active/answering
  ├─ Submit answer → active/deciding
  ├─ Skip → active/deciding
  └─ User ends interview → completed/completed

active/deciding
  ├─ Generate next question → active/answering
  ├─ State machine ends → completed/completed
  └─ Decision fails → active/decision_failed

active/decision_failed
  ├─ Retry → active/deciding
  ├─ Explicitly use fallback question → active/answering
  └─ User ends interview → completed/completed
~~~

---

## 4. Domain-contract adjustments

### 4.1 `InterviewBlueprintV2`

Remove the fixed `question_count=5` semantics and use:

~~~json
{
  "schema_version": "v2",
  "mode": "standard",
  "min_turns": 8,
  "target_turns": 12,
  "max_turns": 16,
  "time_budget_minutes": 30,
  "max_follow_up_depth": 2,
  "max_follow_ups_total": 4,
  "capabilities": [
    {
      "key": "project_depth",
      "label": "Project deep dive",
      "weight": 25,
      "target_evidence": 2,
      "difficulty_curve": ["medium", "hard"],
      "rubric": ["individual contribution", "technical trade-offs", "quantified results"]
    }
  ],
  "evidence_scope": ["resume", "answer", "company_intel"]
}
~~~

Server-side validation:

- `min_turns <= target_turns <= max_turns`.
- Standard-mode defaults are `8 / 12 / 16 / 30 minutes`.
- Capability weights sum to 100.
- Every capability key is unique.
- `max_follow_up_depth <= 2`.
- `max_follow_ups_total < max_turns`.
- The Prompt cannot expand the bounds allowed by the server.

### 4.2 `InterviewMaterials`

Replace the current `GeneratedQuestionSet`'s “exactly five questions” semantics:

~~~json
{
  "capabilities": [
    {
      "capability_key": "project_depth",
      "expected_evidence": ["individual responsibility", "technical decisions", "result metrics"],
      "anchor_questions": ["Describe your individual contribution to the most challenging project you have worked on."],
      "fallback_questions": ["What was the most important technical trade-off in this project?"]
    }
  ]
}
~~~

Rules:

- The number of anchor and fallback questions is determined by the number of capabilities; it is not the total number of real interview rounds.
- Material-related content must cite fact IDs from the current resume version.
- The normal AI path must not play these questions in `ordinal` order.

### 4.3 `NextTurnDecisionV2`

~~~json
{
  "action": "deepen",
  "question": "You mentioned that throughput increased by 40%. What were the baseline and measurement window for this metric?",
  "turn_kind": "follow_up",
  "capability_key": "project_depth",
  "intent": "Verify the definition of the quantified result",
  "expected_points": ["baseline", "measurement window", "individual contribution"],
  "difficulty": "hard",
  "evidence_fact_ids": ["..."],
  "reason": "The current answer gives a result but does not define the metric",
  "coverage_observation": {
    "evidence_quality": 55,
    "resolved": [],
    "unresolved": ["metric definition"]
  }
}
~~~

Allowed actions:

~~~text
deepen
switch_capability
recommend_finish
~~~

Validation:

- `deepen` and `switch_capability` must have a non-empty `question`.
- `recommend_finish` must not contain a next question.
- `deepen` may not leave the current capability and may not exceed follow-up depth or budget.
- `switch_capability` must select a capability key from the blueprint.
- Evidence IDs must belong to the current Session's `resume_version_id`.
- `coverage_observation` may update only the current capability; values are clipped by the server.
- The model cannot directly modify the Session, round count, time budget or final score.

### 4.4 `CapabilityProgress`

~~~json
{
  "project_depth": {
    "asked_turns": 2,
    "follow_up_turns": 1,
    "evidence_count": 1,
    "evidence_quality": 55,
    "coverage_score": 50,
    "last_difficulty": "hard",
    "unresolved_gaps": ["metric definition"]
  }
}
~~~

Update strategy:

- Calculate asked/follow-up counts from actual database Turns or increment them on the server.
- The model may only suggest evidence quality, resolved items and unresolved items.
- Calculate coverage score using a deterministic formula.
- When high-weight capability coverage is insufficient, prioritize switching or further deepening.

---

## 5. Public API adjustments

### 5.1 Interface-change overview

| Current interface | Adjustment | Goal |
| --- | --- | --- |
| `POST /interviews` | Keep it; narrow the request and return a preparing Session | Create an AI interview |
| `GET /interviews` | Keep it; add agent mode / phase / turn count | Interview-record list |
| `GET /interviews/:id` | Keep it; add policy, progress and operation state | Single polling entry point |
| `PUT /interviews/:id/turns/:ordinal/answer` | Change to asynchronous 202 progression | Save an answer and trigger the next question |
| `POST /interviews/:id/turns/:ordinal/skip` | Change to asynchronous 202 progression | Skip and trigger the next question |
| `POST /interviews/:id/complete` | Keep it, explicitly as user-initiated completion | Must not be used for model completion |
| `POST /interviews/:id/answer` | Delete deprecated interface | Avoid two answer semantics |
| `GET /interviews/:id/report` | Keep it; enhance dynamic-Turn fields | Get the report |
| `GET /tasks/:id` | Delete the public interface after migration | Do not expose generic tasks |
| `POST /tasks/:id/retry` | Delete the public interface after migration | Use business-specific retry |
| None | Add preparation retry | `POST /interviews/:id/preparation/retry` |
| None | Add next-round retry | `POST /interviews/:id/next-turn/retry` |
| None | Add report retry | `POST /interviews/:id/report/retry` |
| None | Optional P1 SSE | `GET /interviews/:id/events` |

### 5.2 Create an interview

#### `POST /api/v1/interviews`

Request:

~~~json
{
  "resume_id": "uuid",
  "primary_language": "Java",
  "target_company": "Target company"
}
~~~

Adjustments:

- The first release no longer lets users provide `question_duration_seconds`.
- The backend fixes the role to `backend_development`.
- The mode defaults to `standard`; add it as an optional field only when more modes are introduced.
- If AI is not enabled and this is not an explicit development-demo environment, return `AI_NOT_CONFIGURED`.

Response: `202 Accepted`

~~~json
{
  "id": "uuid",
  "status": "preparing",
  "phase": "preparing",
  "agent_mode": "ai",
  "primary_language": "Java",
  "target_company": "Target company",
  "target_role": "backend_development",
  "policy": {
    "min_turns": 8,
    "target_turns": 12,
    "max_turns": 16,
    "time_budget_minutes": 30
  },
  "operation": {
    "type": "preparation",
    "status": "pending",
    "retryable": false
  },
  "current_turn": null,
  "created_at": "RFC3339"
}
~~~

Do not return `question_set_id` or a generic `task_id` to the frontend.

### 5.3 Get an interview

#### `GET /api/v1/interviews/:id`

Suggested response:

~~~json
{
  "id": "uuid",
  "status": "active",
  "phase": "deciding",
  "agent_mode": "ai",
  "primary_language": "Java",
  "target_company": "Target company",
  "target_role": "backend_development",
  "turn_count": 6,
  "answered_count": 6,
  "skipped_count": 0,
  "current_ordinal": 6,
  "elapsed_seconds": 720,
  "policy": {
    "min_turns": 8,
    "target_turns": 12,
    "max_turns": 16,
    "time_budget_minutes": 30
  },
  "progress": {
    "covered_weight": 45,
    "follow_ups_used": 2,
    "follow_ups_remaining": 2,
    "capabilities": []
  },
  "operation": {
    "type": "next_turn",
    "status": "running",
    "retryable": false,
    "error_code": "",
    "error_summary": ""
  },
  "current_turn": null,
  "turns": []
}
~~~

Compatibility strategy:

- Add `turn_count` to replace the ambiguous `question_count` semantics.
- Delete `question_count` after frontend migration is complete.
- When `phase=deciding`, `current_turn` may be empty, but operation must explain that generation is in progress.
- The Turn list returns only questions that were actually asked.

### 5.4 Submit an answer

#### `PUT /api/v1/interviews/:id/turns/:ordinal/answer`

Request:

~~~json
{
  "answer": "User answer",
  "client_request_id": "uuid"
}
~~~

Response: `202 Accepted`

~~~json
{
  "session_id": "uuid",
  "accepted_ordinal": 6,
  "phase": "deciding",
  "decision_status": "pending",
  "updated_at": "RFC3339"
}
~~~

Constraints:

- `client_request_id` or `Idempotency-Key` must be unique.
- Repeating the same answer for a Turn returns the same decision status.
- A different answer for the same Turn returns `INTERVIEW_TURN_ALREADY_ANSWERED`.
- The answer, Session phase and decision must be committed in one transaction.
- The API does not wait for the model within the request lifecycle.

### 5.5 Skip a question

#### `POST /api/v1/interviews/:id/turns/:ordinal/skip`

- Save `skipped_at`.
- Create a next-turn decision.
- Return the same `202` structure as answer submission.
- Explicitly mark this question as skipped in the Agent input; do not interpret an empty answer as insufficient capability.

### 5.6 User-initiated completion

#### `POST /api/v1/interviews/:id/complete`

Request:

~~~json
{
  "confirm_incomplete": true
}
~~~

Rules:

- This is user-initiated completion, not a model-completion endpoint.
- When `phase=deciding`, return a conflict by default to avoid a race between the in-progress decision and the completion transaction.
- After a second confirmation, the user may cancel a pending decision and end the interview.
- Save `completion_reason=user_ended`.
- An already completed Session returns its current state idempotently.

### 5.7 Business-specific retry

#### `POST /api/v1/interviews/:id/preparation/retry`

- Only allow it when `status=failed` and the latest preparation task failed.
- Create a new preparation attempt and restore `preparing`.

#### `POST /api/v1/interviews/:id/next-turn/retry`

- Only allow it when `status=active && phase=decision_failed`.
- Reuse the original answered Turn and decision ID, and increment attempt.
- Do not save or modify the answer again.

#### `POST /api/v1/interviews/:id/report/retry`

- Only allow it when the interview is complete and the latest report status is failed.
- Do not expose generic task types to the frontend.

### 5.8 P1 SSE

#### `GET /api/v1/interviews/:id/events`

Suggested events:

~~~text
interview.prepared
decision.started
turn.ready
decision.failed
interview.completed
report.ready
report.failed
~~~

MVP can first use short polling through `GET /interviews/:id`; this must not block the main-loop refactor.

### 5.9 Error codes

Add or unify:

~~~text
AI_NOT_CONFIGURED
INTERVIEW_NOT_ACTIVE
INTERVIEW_NOT_ANSWERING
INTERVIEW_DECISION_IN_PROGRESS
INTERVIEW_DECISION_FAILED
INTERVIEW_TURN_ALREADY_ANSWERED
INTERVIEW_TURN_NOT_CURRENT
INTERVIEW_POLICY_REJECTED
NEXT_TURN_OUTPUT_INVALID
NEXT_TURN_PROVIDER_UNAVAILABLE
NEXT_TURN_BUDGET_EXHAUSTED
NEXT_TURN_RETRY_NOT_ALLOWED
~~~

Clients receive only safe summaries; the Provider's original error remains in server-side audit data.

---

## 6. Database migration TODO

Add `00010_agent_runtime_v2.sql`.

### DB-001 Session fields

- [ ] Add `agent_mode text NOT NULL`, restricted to `ai | rule | legacy`.
- [ ] Add `phase text NOT NULL`.
- [ ] Add `policy_version text`.
- [ ] Add `interviewer_prompt_version text`.
- [ ] Add `min_turns`, `target_turns` and `max_turns`.
- [ ] Add `time_budget_minutes`.
- [ ] Add `max_follow_up_depth` and `max_follow_ups_total`.
- [ ] Add `capability_progress jsonb NOT NULL DEFAULT '{}'`.
- [ ] Add `completion_reason text`.
- [ ] Add phase/status CHECK constraints.
- [ ] Add an index for `user_id + status + phase + updated_at`.

### DB-002 Turn snapshot fields

- [ ] Add `intent text NOT NULL DEFAULT ''`.
- [ ] Add `expected_points jsonb NOT NULL DEFAULT '[]'`.
- [ ] Add `difficulty text NOT NULL DEFAULT 'medium'`.
- [ ] Add `evidence_fact_ids jsonb NOT NULL DEFAULT '[]'`.
- [ ] Add `decision_reason text NOT NULL DEFAULT ''`.
- [ ] Add `coverage_observation jsonb NOT NULL DEFAULT '{}'`.
- [ ] Add an optional foreign key from `invocation_id` to `model_invocations(id)`.
- [ ] Retain `source_question_id`, but use it only for anchor or fallback questions.

### DB-003 Next-turn Decision table

- [ ] Create `interview_turn_decisions`:

~~~text
id uuid PK
session_id uuid FK
answered_turn_id uuid FK
next_turn_id uuid FK nullable
status pending/running/succeeded/failed/cancelled
action
attempt
input_hash
model_invocation_id uuid nullable
error_code
error_summary
created_at
started_at
completed_at
updated_at
~~~

- [ ] Add a unique constraint on `answered_turn_id` so one answer produces one logical decision.
- [ ] Add a recovery-scan index for pending/running states.
- [ ] Every update must also validate the Session owner and current phase.

### DB-004 Internal-material compatibility

- [ ] Upgrade `question_sets.blueprint` to v2 and retain the Schema version.
- [ ] Add `material_kind=anchor|fallback` or an equivalent field to `questions`.
- [ ] No longer require exactly five questions.
- [ ] No longer use `questions.ordinal` to determine the real interview order.

### DB-005 Historical-data migration

- [ ] Mark completed Sessions as `agent_mode=legacy`, keeping their records readable.
- [ ] Let old active Sessions continue through the legacy runner, or explicitly archive them before release; do not switch midway to V2.
- [ ] New Sessions always use a v2 policy.
- [ ] The v10 Down migration must explicitly handle active V2 Sessions and must not silently lose decisions.
- [ ] Complete an up/down/up verification with v9 historical samples.

---

## 7. Internal tasks and Worker TODO

### TASK-001 Rename the preparation task

- [ ] Change the business semantics of `question:generate` to `interview:prepare`.
- [ ] During migration, register both the old and new task names in the Worker so old queued messages are not lost.
- [ ] Retain Session, User, Resume and ResumeVersion in the payload.
- [ ] The Worker reads language, company, role and policy from the database to reduce payload drift.
- [ ] Successful output is BlueprintV2, Materials and the first Turn.
- [ ] When AI-mode preparation fails, enter failed; do not fabricate “AI success.”

### TASK-002 Add the next-round task

- [ ] Add `TypeInterviewNextTurn = "interview:next_turn"`.
- [ ] Minimize the payload:

~~~json
{
  "decision_id": "uuid",
  "session_id": "uuid",
  "answered_turn_id": "uuid",
  "user_id": "uuid"
}
~~~

- [ ] On claim, validate the association among user, Session, Turn, decision and phase.
- [ ] Return idempotently for tasks already succeeded/cancelled.
- [ ] Allow controlled takeover of timed-out running tasks.
- [ ] Perform model calls outside the database transaction.
- [ ] Re-lock the Session and validate the state version when applying a decision.
- [ ] Commit the next Turn and progress update in one transaction.
- [ ] A crash before Ack and replay must not insert a duplicate Turn.

### TASK-003 Decision-failure strategy

- [ ] Clearly distinguish retryable Provider errors, output errors, budget errors and permanent configuration errors.
- [ ] Limit automatic retries; never consume the model without bound.
- [ ] After final failure, set the Session to `decision_failed`.
- [ ] The frontend displays a safe error summary and retry entry point.
- [ ] Insert a fallback Turn only when the user chooses “continue with fallback.”
- [ ] Mark fallback Turns with their source and `agent_mode/rule`; never disguise them as real-time AI.

### TASK-004 Recovery scanning

- [ ] The Worker periodically scans timed-out preparing tasks.
- [ ] The Worker periodically scans timed-out pending/running decisions.
- [ ] Recovery actions use the same logical unique key.
- [ ] After the recovery count exceeds the threshold, enter an explicit failure state.

### TASK-005 Report task

- [ ] The report reads Turn snapshots directly.
- [ ] Dynamic main questions and follow-ups use the same scoring path.
- [ ] The report task no longer obtains necessary Rubrics through `source_question_id`.
- [ ] Use controlled parallelism for formal scoring to avoid serial timeout on long interviews.

---

## 8. Agent and Prompt implementation TODO

### AI-001 Blueprint V2

- [ ] Add `blueprint.generate/v2` without modifying v1.
- [ ] Remove the fixed-five-question instructions.
- [ ] Output min/target/max, capability weights, evidence targets and difficulty curve.
- [ ] A Go Validator enforces server-side boundaries.
- [ ] Add tests for invalid rounds, weights, duplicate capabilities and out-of-bounds budgets.

### AI-002 Materials

- [ ] Add `interview.materials/v1`.
- [ ] Output Rubrics, expected evidence, anchors and fallbacks.
- [ ] Add duplicate, capability-coverage and fact-ID validation.
- [ ] The normal flow must not copy Materials in bulk into Turns.

### AI-003 Next-turn V2

- [ ] Add `interview.next_turn/v2`.
- [ ] Implement `NextTurnInputV2` and `NextTurnDecisionV2`.
- [ ] Require `deepen` / `switch_capability` to return a complete question snapshot.
- [ ] Use `recommend_finish` only as a policy input.
- [ ] Context includes policy, progress, elapsed time, current Q&A and the latest three rounds.
- [ ] No longer query candidate questions for normal `next_capability`.

### AI-004 Single ReAct run

- [ ] Refactor the current `RunToolLoop + StructuredRunner` double call.
- [ ] Without tools, accept structured output from the same model call directly.
- [ ] With tools, feed results back and produce the final structured decision once.
- [ ] Allow no more than two tool calls per round.
- [ ] The deadline covers the whole logical run and is coordinated with the Provider's per-call timeout.

### AI-005 Tool scope

- [ ] Change `ResumeFactsTool` construction to use `resume_version_id`, while retaining a user guard.
- [ ] Query SQL must match user, resume and version simultaneously.
- [ ] Limit Company Intel by company, role and result count.
- [ ] Return an empty result when material is not found; do not generate fabricated facts.

### AI-006 Server-side policy

- [ ] Create a pure-Go `InterviewPolicy`.
- [ ] Implement max-turn, time-budget, min-turn and coverage checks.
- [ ] Implement insufficient high-weight capability-coverage checks.
- [ ] Implement follow-up depth and whole-interview follow-up budgets.
- [ ] Model output must not bypass the state machine.
- [ ] Version the Policy and write its version to the Session.

### AI-007 Capability progress

- [ ] Design a deterministic coverage formula.
- [ ] Limit model observations to the current capability.
- [ ] Update asked/follow-up counts after Turn submission.
- [ ] Put evidence quality and unresolved gaps into the next-round context.
- [ ] Add tests for repeated and concurrent updates.

---

## 9. API and generated-code TODO

### API-001 Modify the go-zero contract

- [ ] Update `backend/api/interviewmaster.api`.
- [ ] Delete `AnswerInterviewRequest` and deprecated routes.
- [ ] Adjust Create, Session, Turn and AnswerAccepted types.
- [ ] Add Policy, Progress and Operation types.
- [ ] Add three business-specific retry routes.
- [ ] After migration, delete public TaskPath/GetTask/RetryTask.

### API-002 Update generated artifacts

- [ ] Regenerate routes, handlers and types.
- [ ] Regenerate `backend/api/openapi/interviewmaster.yaml`.
- [ ] Regenerate the Web API Client and Components.
- [ ] Confirm generated artifacts contain no JD, public question-set or public generic-task interfaces.
- [ ] `goctl api validate` passes.

### API-003 Logic layer

- [ ] Create Interview returns a preparation operation, not a task ID.
- [ ] Get Interview aggregates phase, policy, progress, current Turn and operation.
- [ ] Answer/Skip only persist and enqueue; they do not call the model synchronously.
- [ ] Complete and pending decisions are mutually exclusive.
- [ ] Retry recognizes only the latest failed operation.
- [ ] All writes continue to validate `session.user_id`.

### API-004 Concurrency and idempotency

- [ ] Two concurrent answers have only one successful result.
- [ ] The same `client_request_id` returns the same result.
- [ ] A saved answer cannot be overwritten.
- [ ] One answered Turn can be associated with only one decision.
- [ ] next-turn apply uses a state version or strict row lock.

---

## 10. Frontend TODO

### WEB-001 Create page

- [ ] Keep the three core inputs: resume + language + company.
- [ ] Remove `question_duration_seconds`.
- [ ] Default to standard 30 minutes; do not add complex mandatory configuration.
- [ ] When AI is not configured, do not allow starting in AI mode; clearly label development demo mode.

### WEB-002 Interview room

- [ ] Read and display `phase`.
- [ ] `answering` displays the current question and answer input.
- [ ] `deciding` displays “The AI interviewer is analyzing your answer.”
- [ ] `decision_failed` displays the error summary, retry and continue-with-fallback actions.
- [ ] After receiving 202 for answer submission, immediately clear the draft and start polling.
- [ ] Do not assume five total questions in the frontend.
- [ ] Display “N rounds completed”; target rounds are only a range hint.

### WEB-003 Modes and degradation

- [ ] The interview page displays `AI Interview` or `Rule Demo`.
- [ ] A fallback Turn displays “System fallback question” and cannot be labeled as an answer-based follow-up.
- [ ] The record page retains the real Turn kind and generation mode.

### WEB-004 Remove generic task dependence from retries

- [ ] Preparation failure calls preparation retry.
- [ ] Next-question failure calls next-turn retry.
- [ ] Report failure calls report retry.
- [ ] Remove the Web dependency on generic `tasks/:id`.

### WEB-005 SSE (P1)

- [ ] Add EventSource only after polling is stable.
- [ ] Automatically fall back to polling when SSE disconnects.
- [ ] Treat events only as refresh signals; database queries remain authoritative.

---

## 11. Report and record interface TODO

### REPORT-001 Turn Report

- [ ] Add the following to `TurnReportResponse`:

~~~text
turn_kind
capability_key
intent
difficulty
expected_points
generation_mode
~~~

- [ ] Score display must not depend on internal question sets.
- [ ] Dynamic follow-ups and dynamic main questions must both have feedback and a reference answer.

### REPORT-002 Summary

- [ ] Use `turn_count` instead of `question_count` in lists.
- [ ] Do not show a score of 0 when the interview is `completed` but the report is pending.
- [ ] Dashboard aggregates only completed/degraded reports.

---

## 12. Testing and acceptance TODO

### TEST-001 Contract

- [ ] Test valid/invalid Blueprint V2 boundaries.
- [ ] Test the three NextTurnDecision V2 actions.
- [ ] Reject a non-terminal action without `question`.
- [ ] Reject unknown capability keys, unauthorized fact IDs and out-of-range observations.
- [ ] Use table-driven tests for Policy finish conditions.

### TEST-002 Agent

- [ ] When an answer contains a concrete metric, generate a follow-up that verifies the metric definition.
- [ ] When the current capability has sufficient evidence, switch capabilities and generate a new question.
- [ ] Reject `recommend_finish` before min turns is reached.
- [ ] Force completion at max turns or the time budget.
- [ ] Without tool calls, issue only one final model request.
- [ ] Make no more than two tool calls.

### TEST-003 Data and concurrency

- [ ] Concurrent duplicate answers.
- [ ] Worker crash recovery after answer commit.
- [ ] Replay after next-Turn commit but before Ack.
- [ ] Retry the latest failed decision in place.
- [ ] An old decision cannot overwrite new state.
- [ ] Reject cross-user and cross-resume-version tool access.

### TEST-004 API

- [ ] Create returns 202 preparing.
- [ ] Answer/Skip returns 202 deciding.
- [ ] Get Interview changes from deciding to answering.
- [ ] Handle Complete conflicts with deciding.
- [ ] Enforce the state constraints of the three business-specific retries.
- [ ] Public OpenAPI no longer contains generic task query and retry.

### TEST-005 Frontend

- [ ] Page states for answering / deciding / decision_failed / completed.
- [ ] AI and rule-mode labels.
- [ ] Display and submit correctly beyond five rounds.
- [ ] Refreshing the page does not lose deciding state.
- [ ] Polling or SSE receives the new Turn.

### TEST-006 Real-model closed loop

- [ ] Configure a real OpenAI-compatible Provider.
- [ ] Use two different resumes to complete at least two standard interviews.
- [ ] Each interview naturally exceeds five rounds.
- [ ] The next question responds to the previous answer.
- [ ] Every answered Turn has a corresponding `model_invocations` record.
- [ ] Complete preparation, dynamic Q&A, state-machine completion and the report loop.
- [ ] Record latency, Tokens, cost, failure rate and degradation count.

---

## 13. Release and compatibility TODO

### RELEASE-001 Dual-version compatibility

- [ ] New Sessions use policy v2.
- [ ] Historical completed Sessions remain read-compatible.
- [ ] Historical active Sessions explicitly use legacy or are archived; do not mix modes dynamically.
- [ ] During the transition, the Worker supports the old preparation-task name.

### RELEASE-002 Gradual rollout

- [ ] Add a server-side `agent_runtime_v2` rollout switch.
- [ ] Enable it for internal accounts first.
- [ ] Monitor next-turn success rate, P95, average tool calls and per-interview cost.
- [ ] If the failure rate exceeds the threshold, stop creating new V2 Sessions without modifying historical Sessions.

### RELEASE-003 Rollback

- [ ] Back up the database before release.
- [ ] Keep V2 tables and data readable when rolling back the application.
- [ ] Do not turn a started V2 Session back into a fixed question set.
- [ ] If necessary, stop new interviews while allowing completed records and reports to remain accessible.

---

## 14. Recommended implementation batches

### Batch A: contracts and database

- [ ] AI-001 Blueprint V2.
- [ ] AI-002 Materials.
- [ ] AI-003 Next-turn V2 Contract.
- [ ] DB-001–DB-005.
- [ ] TEST-001.

Definition of done: the migration can be rolled back and all domain contracts pass tests, but existing traffic has not yet been switched.

### Batch B: next-turn Worker and state machine

- [ ] TASK-002–TASK-004.
- [ ] AI-004–AI-007.
- [ ] API-003–API-004.
- [ ] TEST-002–TEST-003.

Definition of done: under Fake Model, a dynamic interview can reliably exceed five rounds and recover from a crash.

### Batch C: public interfaces and frontend

- [ ] API-001–API-002.
- [ ] WEB-001–WEB-004.
- [ ] TEST-004–TEST-005.

Definition of done: after submitting an answer, the user sees deciding and then a new question generated from that answer.

### Batch D: reporting and real models

- [ ] TASK-005.
- [ ] REPORT-001–REPORT-002.
- [ ] TEST-006.

Definition of done: a real Provider completes preparation, at least eight rounds of dynamic Q&A, completion and a full scoring report.

### Batch E: gradual rollout and cleanup

- [ ] RELEASE-001–RELEASE-003.
- [ ] Delete the legacy answer interface.
- [ ] Delete the public generic Task API.
- [ ] Delete the fixed-five-question Prompt and normal playback path.
- [ ] Reassess SSE in P1.

---

## 15. File-level adjustment index

| File/directory | Main adjustment |
| --- | --- |
| `backend/api/interviewmaster.api` | New response types, 202 Answer, business retries, delete old interfaces |
| `backend/api/openapi/interviewmaster.yaml` | Regenerate |
| `backend/internal/platform/ai/contract/contract.go` | BlueprintV2, Materials, NextTurnDecisionV2, Policy |
| `backend/internal/platform/ai/react.go` | Single ReAct final structured output |
| `backend/internal/platform/ai/tools/tools.go` | Bind `resume_version_id` |
| `backend/internal/aiworkflow/interviewer.go` | Change to next-turn v2 |
| `backend/internal/aiworkflow/workflow.go` | Split blueprint/materials/evaluation/report |
| `backend/apps/api/internal/logic/workspace/createinterviewlogic.go` | phase, asynchronous Answer, business retries |
| `backend/apps/api/internal/logic/workspace/interviewer.go` | State machine and apply decision; delete normal candidate-question selection |
| `backend/apps/worker/internal/tasks/questiongen.go` | Change to interview preparation |
| `backend/apps/worker/internal/tasks/nextturn.go` | Add |
| `backend/apps/worker/internal/tasks/reportgen.go` | Use Turn snapshots |
| `backend/internal/tasks/question.go` | Rename preparation task and add compatibility |
| `backend/internal/tasks/nextturn.go` | Add payload |
| `backend/prompts/blueprint.generate/v2/` | Add |
| `backend/prompts/interview.materials/v1/` | Add |
| `backend/prompts/interview.next_turn/v2/` | Add |
| `backend/migrations/00010_agent_runtime_v2.sql` | Session, Turn and Decision |
| `web/src/shared/api/*` | Regenerate and adjust service wrappers |
| `web/src/features/interviews/InterviewRoomPage.tsx` | phase, deciding, retry and mode label |
| `web/src/features/interviews/InterviewRecordPage.tsx` | Dynamic question snapshots and generation mode |

---

## 16. Definition of Done

Agent V2 may be marked complete only when all of the following conditions are met:

- [ ] The normal path no longer takes the next question from a fixed candidate-question queue.
- [ ] After every answer, a real model generates a specific next question.
- [ ] A standard interview has at least 8 rounds, can naturally reach the target of 12, and never exceeds 16 rounds.
- [ ] The state machine, not the model, decides when to end.
- [ ] Answer submission, deciding, failure retry and crash recovery are all persisted.
- [ ] Dynamic Turns save a complete scoring snapshot.
- [ ] Resume tools are strictly bound to the Session's resume version.
- [ ] AI disablement or degradation is clearly visible in the frontend.
- [ ] The public API exposes neither question sets nor generic tasks.
- [ ] `model_invocations` is non-zero in real-Provider E2E.
- [ ] Backend tests, frontend tests, builds, migration up/down/up and real HTTP E2E all pass.
- [ ] `agent-design.md`, OpenAPI, this document and the final code remain consistent.
