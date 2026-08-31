# Invite-only Launch TODO (2026-08-28)

> Scope: issue accounts to acquaintances/beta users, close public registration, run the main path with a real model, and leave the minimum proof.
> This is not public registration, not large-scale operations, and not another round of Agent architecture work.
> The design still follows [`agent-design.md`](agent-design.md); the V2 historical interface notes are in [`archive/agent-architecture-api-refactor-todo_0814.md`](archive/agent-architecture-api-refactor-todo_0814.md) for historical reference only.

### Status markers

- `[ ]` Not done
- `[-]` Skeleton exists, but not accepted to the invite-only standard
- `[x]` Completed; do not redo during the invite-only phase

### Definition of done

All of the following must be true before we can say “ready to start inviting”:

1. Strangers cannot register themselves in production; every new account must register with a valid invite code.
2. An invited user can complete: registration/login → resume upload → start interview → at least 8 real-model Q&A turns → view the report; `model_invocations` is non-zero.
3. When AI is turned off, the page clearly shows rule-based degradation and does not pretend to be an AI interview.
4. Keys and database passwords are not in Git; when something goes wrong, AI can be turned off according to [`runbooks/ai.md`](runbooks/ai.md).

---

## 0. Already available (do not redo)

The product path is approximately 90% complete, and the intelligent-layer main path is already in the code:

- [x] The user unit is “one interview”; the frontend no longer manages JD / question sets.
- [x] Resume upload and Tika parsing; when AI is enabled, it uses `resume.extract`.
- [x] The preparation phase asynchronously generates the blueprint, internal materials, and first question (database v10, `interview:prepare`).
- [x] After each answer, the Worker runs `interview:next_turn`; the state machine decides when to finish instead of letting the model stop on its own.
- [x] Interview-room bubble UI; history remains available during `deciding` / `decision_failed`, and the composer stays fixed at the bottom.
- [x] Reports are scored from Turn snapshots, not word count.
- [x] `model_invocations` audit, per-user call/cost limits, and a Prompt version catalog.
- [x] Fake Model + Golden sample unit tests (`backend/internal/aieval`, etc.); CI runs `go test ./...` and frontend lint/test/build.
- [x] Rule-based degradation: `IM_AI_ENABLED=false` in `.env` runs without keys; the page labels it “Rule-degraded interview.”
- [x] Frontend Mock (`pnpm mock:web`, port 3001) is only for viewing the UI without a backend and **cannot** replace real-model acceptance.

Capabilities needed only for public registration (email verification, CAPTCHA, Argon2id, virus scanning, full load testing, backup drills, Prometheus/Alertmanager) are **not part of this checklist** and will be scheduled in a separate public-registration phase checklist.

---

## 1. P0 gate: keep strangers out

Current state: the registration API uniformly requires an invite code, and the website registration entry point is now “Invite-code registration.” Invite codes are stored as hashes, and available uses are decremented atomically inside the registration transaction.

The project's only invite scheme is **invite codes**; registration flags and email whitelists will not be implemented in parallel.

### 1.1 Invite code

- [x] Invite-code table: code, creator, expiration time, available uses or one-time use, used count, and optional bound email.
- [x] Add required `invite_code` to `POST /auth/register`; invalid/expired/exhausted codes return a unified error.
- [x] Decrement uses in the same transaction after successful use to prevent concurrent over-issuance.
- [x] Add an invite-code input to frontend `/register`; unify the marketing-page entry point as “Invite-code registration.”
- [x] Local/seed setup: document how to insert the first code (SQL). Production codes must not enter the repository.
- [ ] Tests: no code, wrong code, expired code, exhausted code, successful registration with a valid code, and concurrent use of the same code.

Gate acceptance: requests without a code or without a code on the allowed list are rejected by both the API and UI; existing accounts can still log in.

---

## 2. P0 manual acceptance: complete one real-model run

Before automation is ready, someone must run the flow once with the **production-equivalent configuration**. Rule mode and Mock do not count.

- [ ] `.env`: `IM_AI_ENABLED=true`; Provider, Base URL, model name, and key come from the local machine or server environment and are not committed to Git.
- [ ] Invited account: upload resume → parsing completes → choose language and target company → start interview.
- [ ] If preparation fails, the user sees failure/retry and there is no false success.
- [ ] Complete at least the default minimum number of turns (8 in standard mode), including at least one follow-up or capability switch.
- [ ] Generate the report after finishing; per-question comments correspond to the real Q&A.
- [ ] Check the database: the `model_invocations` count for this `session_id` is > 0 and matches the provider / model / prompt_key / version.
- [ ] Start another interview with `IM_AI_ENABLED=false`; confirm the degradation notice appears and that no report is written as if it were AI-generated.
- [ ] Record configuration details for this run (without keys) in a local or private note; public documentation should point only to [`local-setup.md`](local-setup.md) and [`runbooks/ai.md`](runbooks/ai.md).

---

## 3. P1 proof: CI can catch regressions

Invite-only testing can initially rely on the manual run in section 2, but changes to Prompts and the state machine can silently break the path. Implement the following in order of value:

### 3.1 Golden as a gate, not merely “a test file exists”

Current state: `backend/internal/aieval` and related packages already have Fake Model + `testdata/golden/v1`; CI already runs `go test ./...`. What is missing is a hard gate that blocks merges below an agreed baseline, plus real Provider samples.

- [ ] Agree on the invite-only minimum set: empty answer, off-topic answer, Prompt Injection, normal deepening, and a finish recommendation rejected by the state machine. Failure must be an explicit error or controlled degradation and must not rewrite the business state machine.
- [ ] Keep this package on the mandatory CI path (if it already is, only add missing samples; do not create a parallel test suite).
- [ ] **Do not do this:** a large manually labeled comparison set (wait until public registration).

### 3.2 Worker integration (real PG + Redis + queue)

Current state: many unit tests exist; integration tests for duplicate delivery, crash replay, idempotency, and dead letters are missing.

- [ ] Minimum integration: `interview:prepare` succeeds once; replaying the same payload does not insert a duplicate first question.
- [ ] Minimum integration: if the process crashes before acknowledging `interview:next_turn` and the task is delivered again, no duplicate Turn appears.
- [ ] Use a Compose test database or CI service container; do not pretend to have a queue inside unit tests.

### 3.3 Playwright main path (rule mode is acceptable)

Current state: `web/e2e/marketing.spec.ts` covers only website copy and redirecting an unauthenticated user to `/login`. CI **does not** run Playwright.

- [ ] Rule mode or Mock: register (with an invite code, if 1.1 is already done) → log in → upload a resume (or fixture) → enter the interview room and see the first question.
- [ ] After submitting one answer, `deciding` appears, followed by the next question or an explicit failure/retry state.
- [ ] CI does not need to call a real model every time (expensive and unstable). Keep the real model in section 2 or in an optional smoke test.

### 3.4 Optional: real Provider smoke

- [ ] Use a separate job or manual script; keys come only from CI Secrets / the local environment.
- [ ] Assert that preparation plus at least one `next_turn` produces non-zero `model_invocations`.
- [ ] Failure does not block ordinary PRs during the invite-only phase, but it must run before a release.

---

## 4. P1 operations: one person can keep watch

An independent SRE is not required. The goal is “when the model is down, know how to turn it off; keys do not enter the database.”

- [ ] Use Compose (or an equivalent single-machine setup) for API / Worker / Web / Postgres / Redis / MinIO / Tika; data lives in volumes, with a basic local-backup habit (copying the volume is enough; formal drills belong to public registration).
- [ ] Keep the environment-variable list aligned with [`.env.example`](../../.env.example); production values live only on the server or in a secret manager, and `.env` is not committed.
- [ ] The on-call person can independently follow [`runbooks/ai.md`](runbooks/ai.md): turn off `IM_AI_ENABLED`, inspect `error_code`, and query `model_invocations` (the table contains no answer body).
- [ ] Keep a record of invite-code issuance and revocation (spreadsheet or database query), so a code pasted into a public group can be withdrawn.
- [ ] Keep marketing copy consistent with the gate: before public beta opens, do not say “free registration gives immediate access.”

Already available and sufficient for the invite-only phase:

- [x] `GET /api/v1/health`, `/ready`, and `/metrics` skeletons.
- [x] Structured logging and an OTel skeleton (whether enabled by default depends on the actual configuration).
- [x] User-settings export/delete account (the major compliance work before public registration is not expanded in this checklist).

---

## 5. Recommended order

~~~text
1. Gate (section 1, invite code)
2. Full real-model manual path (section 2) — do not send invitations without this step
3. Golden gap samples + keep CI coverage (3.1)
4. Playwright rule-mode main path (3.3)
5. Worker idempotency integration (3.2)
6. Single-machine deployment and one Runbook walkthrough (section 4)
7. Add real Provider smoke if there is capacity (3.4)
~~~

Do not pull Argon2id, virus scanning, Embedding/RAG from [`archive/launch-todo.md`](archive/launch-todo.md) into this round.

---
