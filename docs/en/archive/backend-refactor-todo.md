# InterviewMaster Backend Refactor TODO and API Contract

> **Archived (2026-08-28).** 07-29 contract draft; the current API is defined by `backend/api/interviewmaster.api` in the repository.
> Document status (when written): P0/P1 completed and accepted
> Updated: 2026-07-29
> Requirements baseline: [`frontend-refactor-requirements.md`](./frontend-refactor-requirements.md)
> Frontend tasks: [`frontend-refactor-todo.md`](./frontend-refactor-todo.md)
> Current contract source: `backend/api/interviewmaster.api`

## 1. Goals

This document defines the backend work and HTTP API contract required for the formal frontend productization. The goals are:

1. Complete resource management, full interview interaction, dashboard, task center, and formal-session interfaces.
2. Upgrade existing interfaces so pagination, errors, times, and status representations remain consistent.
3. Guarantee user-data isolation, historical-interview consistency, and traceability of asynchronous tasks.
4. Ensure `.api`, Go handlers, OpenAPI, and the TypeScript SDK are always generated from the same contract.

Beta company interview-intelligence and ASR interfaces remain available but do not enter the formal product navigation in this round.

## 2. Contract and development conventions

### 2.1 Single source of truth

After an interface review is approved, update this file first:

```text
backend/api/interviewmaster.api
```

Then regenerate, in order:

1. Go handlers, logic, and types.
2. `backend/api/openapi/interviewmaster.yaml`.
3. The TypeScript SDK in `web/src/shared/api/generated/`.

Do not maintain hand-written DTOs duplicated from `.api` in Go, OpenAPI, or the frontend over the long term.

### 2.2 Status markers

- `[ ]` Not started.
- `[-]` In progress.
- `[x]` Completed and accepted.
- `[!]` Requires a product or data-migration decision.

### 2.3 HTTP conventions

- Formal interfaces uniformly use the `/api/v1` prefix.
- Except for registration, login, session refresh, logout, and health checks, business interfaces require Access Token authentication.
- Access Token uses `Authorization: Bearer <token>`.
- Successful creation returns `201 Created`.
- Successful ordinary reads, updates, and actions return `200 OK`.
- An accepted asynchronous task returns `202 Accepted`.
- Successful deletion and logout return `204 No Content`.
- JSON fields uniformly use `snake_case`.
- All times use UTC RFC 3339, for example `2026-07-29T08:30:00Z`.
- All IDs are UUID strings.
- Empty arrays must be returned as `[]`, never `null`.
- Nullable scalar values use `null` or an omitted field; the same field must be consistent across all interfaces.

### 2.4 Pagination conventions

All resource lists use:

```ts
type PageResponse<T> = {
  items: T[];
  page: number;
  page_size: number;
  total: number;
};
```

Query parameters:

| Parameter   | Type    | Default       | Rule                                  |
| ----------- | ------- | ------------- | ------------------------------------- |
| `page`      | integer | `1`           | Minimum 1                             |
| `page_size` | integer | `20`          | 1–100                                 |
| `sort`      | string  | Resource default | Accept only enums declared by the interface |

Paginated queries must provide a stable secondary sort key, for example `updated_at DESC, id DESC`, to avoid duplicate or missing entries across pages.

### 2.5 Error structure

All non-2xx JSON errors uniformly return:

```ts
type ErrorResponse = {
  code: string;
  message: string;
  request_id?: string;
  details?: {
    fields?: Record<string, string[]>;
    [key: string]: unknown;
  };
};
```

Example:

```json
{
  "code": "INTERVIEW_HAS_UNANSWERED_TURNS",
  "message": "仍有未回答的问题，请确认后再完成面试",
  "request_id": "req_01J...",
  "details": {
    "ordinals": [2, 4]
  }
}
```

HTTP status rules:

| Status | Use case                                      |
| ------ | --------------------------------------------- |
| `400`  | Invalid request format, field, or query parameter |
| `401`  | Unauthenticated, invalid Access Token, or expired session |
| `403`  | Authenticated but not authorized for the operation |
| `404`  | Resource does not exist in the current user’s scope |
| `409`  | Referenced resource, state conflict, duplicate completion, or another business conflict |
| `413`  | Uploaded file is too large                    |
| `415`  | Unsupported file type                         |
| `429`  | Rate limit exceeded                           |
| `500`  | Uncategorized server error                   |
| `503`  | Database, Redis, object storage, or model service unavailable |

Production environments must not put SQL, stack traces, object-storage paths, raw model exceptions, or other sensitive details into `message` or `details`.

### 2.6 User isolation

- Every resource query and mutation must include the current `user_id` condition.
- Access to another user’s resource uniformly returns `404`, preventing disclosure of whether the resource exists.
- When creating related resources, verify that all resources belong to the current user.
- Lists, dashboards, and task statistics must not aggregate across users.

## 3. Common data types

The following TypeScript is used only to describe the JSON contract clearly; final types come from `.api` generation.

### 3.1 Enums

```ts
type ResumeStatus =
  "draft" | "uploading" | "pending" | "processing" | "completed" | "failed";

type TaskStatus = "pending" | "running" | "succeeded" | "failed";
type QuestionSetStatus = "ready" | "archived";
type InterviewStatus = "draft" | "active" | "completed" | "abandoned";
type InterviewTurnState = "unstarted" | "answering" | "answered" | "skipped";
```

When adding an enum value, update `.api`, OpenAPI, frontend status mappings, and tests together.

### 3.2 User and authentication

```ts
type UserResponse = {
  id: string;
  email: string;
  display_name: string;
};

type AuthResponse = {
  access_token: string;
  token_type: "Bearer";
  expires_in: number;
  user: UserResponse;
};
```

### 3.3 Resume

```ts
type ResumeSummaryResponse = {
  id: string;
  title: string;
  status: ResumeStatus;
  version_id?: string;
  file_name?: string;
  created_at: string;
  updated_at: string;
};

type ResumeFactResponse = {
  type: string;
  key: string;
  value: unknown;
  excerpt: string;
  confidence: number;
};

type ResumeDetailResponse = ResumeSummaryResponse & {
  version_no?: number;
  content_type?: string;
  size_bytes?: number;
  uploaded_at?: string;
  processed_at?: string;
  parse_error?: string;
  facts: ResumeFactResponse[];
};
```

### 3.4 JD

```ts
type JobDescriptionResponse = {
  id: string;
  company?: string;
  title: string;
  content: string;
  capabilities: string[];
  created_at: string;
  updated_at: string;
};
```

### 3.5 Question sets

```ts
type QuestionResponse = {
  id: string;
  ordinal: number;
  question: string;
  intent: string;
  expected_points: string[];
  follow_up_hint?: string;
};

type QuestionSetSummaryResponse = {
  id: string;
  resume: {
    id: string;
    title: string;
  };
  job_description?: {
    id: string;
    company?: string;
    title: string;
  };
  target_role?: string;
  status: QuestionSetStatus;
  question_count: number;
  source_question_set_id?: string;
  created_at: string;
  updated_at: string;
};

type QuestionSetDetailResponse = QuestionSetSummaryResponse & {
  questions: QuestionResponse[];
};
```

### 3.6 Interviews

```ts
type InterviewTurnResponse = {
  ordinal: number;
  question: string;
  answer?: string;
  state: InterviewTurnState;
  started_at?: string;
  answered_at?: string;
  skipped_at?: string;
  time_spent_seconds: number;
};

type InterviewSummaryResponse = {
  id: string;
  title: string;
  status: InterviewStatus;
  question_set?: {
    id: string;
    target_role?: string;
  };
  resume: {
    id: string;
    title: string;
  };
  question_count: number;
  answered_count: number;
  skipped_count: number;
  current_ordinal: number;
  overall_score?: number;
  started_at?: string;
  completed_at?: string;
  duration_seconds: number;
  created_at: string;
  updated_at: string;
};

type InterviewSessionResponse = InterviewSummaryResponse & {
  job_description?: {
    id: string;
    company?: string;
    title: string;
  };
  question_duration_seconds: number;
  turns: InterviewTurnResponse[];
};
```

`time_spent_seconds` is a non-negative integer calculated or persisted by the server when responding. For an active question, it may grow with the current time; the frontend may display continuous timing between two server synchronizations.

### 3.7 Reports

```ts
type TurnReportResponse = {
  ordinal: number;
  question: string;
  answer?: string;
  score: number;
  critique: string;
  golden_answer: string;
  evidence: string[];
};

type InterviewReportResponse = {
  id: string;
  session_id: string;
  status: "completed";
  overall_score: number;
  strengths: string[];
  improvements: string[];
  next_steps: string[];
  quality_passed: boolean;
  turns: TurnReportResponse[];
  created_at: string;
  updated_at: string;
};
```

### 3.8 Asynchronous tasks

```ts
type TaskReferenceResponse = {
  type: "resume" | "resume_version" | "interview" | "other";
  id: string;
  title?: string;
};

type TaskErrorResponse = {
  code: string;
  message: string;
};

type TaskResponse = {
  id: string;
  type: string;
  status: TaskStatus;
  progress: number;
  reference: TaskReferenceResponse;
  error?: TaskErrorResponse;
  result?: unknown;
  retry_of_task_id?: string;
  created_at: string;
  updated_at: string;
  completed_at?: string;
};

type TaskAcceptedResponse = {
  task_id: string;
  status: TaskStatus;
};
```

## 4. Input-validation baseline

| Field | Rule |
| ----- | ---- |
| Email | Trim leading/trailing spaces, lowercase, valid email, maximum 254 characters |
| Password | 8–72 characters; never log plaintext |
| Display name | Trim leading/trailing spaces, 1–80 characters |
| Resource title | Trim leading/trailing spaces, 1–120 characters |
| Company name | Optional; trim leading/trailing spaces, maximum 120 characters |
| JD body | Trim leading/trailing spaces, 20–50,000 characters |
| Target role | Optional; trim leading/trailing spaces, maximum 120 characters |
| Interview answer | Reject all-whitespace input; 1–20,000 characters |
| Resume file | PDF, DOCX, TXT; 1 byte–20 MiB |
| Question body | 1–2,000 characters |
| Assessment intent | 1–1,000 characters |
| Expected answer points | At most 20 items, each 1–500 characters |
| Follow-up hint | Optional; maximum 1,000 characters |

Put field errors in `details.fields`, for example:

```json
{
  "code": "VALIDATION_ERROR",
  "message": "提交内容有误，请检查后重试",
  "details": {
    "fields": {
      "content": ["JD 正文至少需要 20 个字符"]
    }
  }
}
```

## 5. P0: Authentication and user interfaces

### BE-AUTH-01 Registration

```http
POST /api/v1/auth/register
```

Request:

```ts
type RegisterRequest = {
  email: string;
  password: string;
  display_name: string;
};
```

Response: `201 AuthResponse`, with a Refresh Cookie set at the same time.

Rules:

- Keep email unique after normalization.
- Store passwords using the project-approved strong hashing algorithm.
- Establish a login session immediately after successful registration.
- Return `409 EMAIL_ALREADY_REGISTERED` when the email is already in use.

### BE-AUTH-02 Login

```http
POST /api/v1/auth/login
```

Request:

```ts
type LoginRequest = {
  email: string;
  password: string;
};
```

Response: `200 AuthResponse`, with a Refresh Cookie set at the same time.

Rules:

- Return `401 AUTH_INVALID_CREDENTIALS` for both unknown email and incorrect password.
- Do not disclose whether an account exists through error copy.
- Rate-limit the login interface by IP and account.

### BE-AUTH-03 Refresh session

```http
POST /api/v1/auth/refresh
```

Request: no JSON Body; read the session from the HttpOnly Refresh Cookie.

Response: `200 AuthResponse`, rotating the Refresh Cookie.

Rules:

- Persist Refresh Tokens only in hashed form.
- Default validity is 30 days; the final value is determined by server configuration.
- Cookie attributes: `HttpOnly`, `Secure` in production, `SameSite=Lax`, `Path=/api/v1/auth`.
- Rotate the Token on every refresh; invalidate the old Token immediately.
- Return `401 AUTH_SESSION_INVALID` for revoked, expired, or replayed tokens.

### BE-AUTH-04 Logout

```http
POST /api/v1/auth/logout
```

Response: `204 No Content`.

Rules:

- Revoke the current Refresh Session.
- Clear the Refresh Cookie.
- Repeated logout remains idempotent.

### BE-USER-01 Current user

```http
GET /api/v1/me
```

Response: `200 UserResponse`.

### BE-USER-02 Update profile

```http
PATCH /api/v1/me
```

Request:

```ts
type UpdateMeRequest = {
  display_name: string;
};
```

Response: `200 UserResponse`.

### BE-USER-03 Change password

```http
POST /api/v1/auth/change-password
```

Request:

```ts
type ChangePasswordRequest = {
  current_password: string;
  new_password: string;
};
```

Response: `204 No Content`.

Rules:

- Return `400 CURRENT_PASSWORD_INCORRECT` when the current password is wrong.
- Return `400 PASSWORD_UNCHANGED` when the new and old passwords are identical.
- After a successful change, revoke all other Refresh Sessions for the user except the current session.

### Authentication data TODO

- [x] Add a Refresh Session table containing at least the user, Token hash, expiry, revocation, creation time, and last-used time.
- [x] Add indexes for lookup by user and Token hash.
- [x] Use a short-lived Access Token and return its exact duration through `expires_in`.
- [x] Add tests for refresh, replay, revocation, and multi-user isolation.

## 6. P0: Resume interfaces

### BE-RESUME-01 Create upload credential

```http
POST /api/v1/resumes/uploads
```

Request:

```ts
type CreateResumeUploadRequest = {
  title: string;
  file_name: string;
  content_type:
    | "application/pdf"
    | "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
    | "text/plain"
    | "application/octet-stream";
  size_bytes: number;
};
```

Response:

```ts
type CreateResumeUploadResponse = {
  resume_id: string;
  version_id: string;
  upload_url: string;
  upload_headers: Record<string, string>;
  expires_at: string;
};
```

Status: `201 Created`.

Rules:

- The server validates extension, declared MIME, and size together; `application/octet-stream` is accepted only as a compatibility value when the browser lacks a valid MIME for a legitimate extension.
- After upload, check the actual file type; do not trust only the extension and client declaration.
- Object Keys must be in the current user’s namespace.
- Presigned URLs are valid for 15 minutes by default; the final value is defined by `expires_at`.
- Do not return internal object-storage credentials.

Errors:

- `413 RESUME_FILE_TOO_LARGE`
- `415 RESUME_FILE_TYPE_UNSUPPORTED`

### BE-RESUME-02 Complete upload and start parsing

```http
POST /api/v1/resumes/:id/versions/:versionId/complete
```

Response: `202 TaskAcceptedResponse`.

Rules:

- Verify that both Resume and Version belong to the current user and are related to each other.
- Verify that the object actually exists and that its size matches the declared value.
- Set the resume status to `pending` and create a `resume.parse` task.
- If the same version already has a `pending/running/succeeded` task, return the existing task for idempotency.
- Set the resume to `completed` after success; set it to `failed` and save a safe error summary after failure.

### BE-RESUME-03 Resume list

```http
GET /api/v1/resumes
```

Query parameters:

| Parameter   | Allowed values                         |
| ----------- | -------------------------------------- |
| `status`    | ResumeStatus, repeatable or comma-separated |
| `page`      | Positive integer                       |
| `page_size` | 1–100                                 |
| `sort`      | `updated_at_desc`, `updated_at_asc`   |

Response: `200 PageResponse<ResumeSummaryResponse>`.

> The current interface returns an array directly and must be upgraded to the unified pagination structure.

### BE-RESUME-04 Resume details

```http
GET /api/v1/resumes/:id
```

Response: `200 ResumeDetailResponse`.

Rules:

- Sort Facts stably by `type` and creation order.
- Return only a cleaned, user-understandable summary in `parse_error`.
- Return `facts: []` when parsing has not completed or there are no facts.

### BE-RESUME-05 Rename

```http
PATCH /api/v1/resumes/:id
```

Request:

```ts
type UpdateResumeRequest = {
  title: string;
};
```

Response: `200 ResumeSummaryResponse`.

### BE-RESUME-06 Delete

```http
DELETE /api/v1/resumes/:id
```

Response: `204 No Content`.

Rules:

- Check related question sets and interviews before deletion.
- Return `409 RESUME_IN_USE` when related resources exist, with reference counts in `details`.
- Do not rely on database-level cascading to silently delete historical question sets, interviews, or reports.
- When there are no relations, delete the database record synchronously; if object-file deletion fails, enter a retryable cleanup mechanism without blocking the completed database deletion.
- If the resource does not exist, still return `204` for delete idempotency.

Conflict example:

```json
{
  "code": "RESUME_IN_USE",
  "message": "这份简历已用于题集或面试，暂时不能删除",
  "details": {
    "question_set_count": 2,
    "interview_count": 1
  }
}
```

### BE-RESUME-07 Reparse

```http
POST /api/v1/resumes/:id/reparse
```

Response: `202 TaskAcceptedResponse`.

Rules:

- Allow only when the current-version file exists.
- Return the existing task when a pending/running parsing task already exists.
- Do not delete current facts before the new task starts; replace them transactionally after success, and retain old facts while marking the status `failed` after failure.

### Resume TODO

- [x] Upgrade the list with pagination and filters.
- [x] Add version number, type, size, processing time, error, and confidence to details.
- [x] Implement rename, deletion with relationship checks, and reparsing.
- [x] Complete interface idempotency handling.
- [x] Add object-storage consistency and multi-user isolation tests.

## 7. P0: JD interfaces

### BE-JOB-01 Create

```http
POST /api/v1/job-descriptions
```

Request:

```ts
type CreateJobDescriptionRequest = {
  company?: string;
  title: string;
  content: string;
};
```

Response: `201 JobDescriptionResponse`.

Rules:

- Extract capability tags on creation.
- Do not fabricate tags when capability extraction fails; return an empty array with a traceable error, or return a clear overall error.
- Deduplicate identical capability tags while preserving a stable order.

### BE-JOB-02 List

```http
GET /api/v1/job-descriptions
```

Query parameters:

| Parameter   | Allowed values                       |
| ----------- | ------------------------------------ |
| `page`      | Positive integer                     |
| `page_size` | 1–100                               |
| `sort`      | `updated_at_desc`, `updated_at_asc` |

Response: `200 PageResponse<JobDescriptionResponse>`.

> The current interface returns an array directly and must be upgraded to the unified pagination structure.

### BE-JOB-03 Details

```http
GET /api/v1/job-descriptions/:id
```

Response: `200 JobDescriptionResponse`.

### BE-JOB-04 Update

```http
PATCH /api/v1/job-descriptions/:id
```

Request:

```ts
type UpdateJobDescriptionRequest = {
  company?: string;
  title?: string;
  content?: string;
};
```

Response: `200 JobDescriptionResponse`.

Rules:

- Provide at least one field.
- Re-extract capability tags when `title` or `content` changes.
- Complete the update and tag replacement in the same transaction.

### BE-JOB-05 Delete

```http
DELETE /api/v1/job-descriptions/:id
```

Response: `204 No Content`.

Rules:

- Preserve generated content in question sets and interviews, setting `job_description_id` to null.
- Deletion must not rewrite or regenerate existing questions, answers, or reports.
- If the resource does not exist, still return `204` for delete idempotency.

### JD TODO

- [x] Upgrade the list with pagination.
- [x] Implement details, update, and deletion.
- [x] Add tag-re-extraction tests after content updates.
- [x] Verify that historical question sets and interviews remain readable after deletion.

## 8. P0: Question-set interfaces

### BE-QSET-01 Generate question set

```http
POST /api/v1/question-sets
```

Request:

```ts
type CreateQuestionSetRequest = {
  resume_id: string;
  job_description_id?: string;
  target_role?: string;
};
```

Response: `201 QuestionSetDetailResponse`.

Rules:

- A resume is required and its status must be `completed`.
- JD is optional; when provided, it must belong to the current user.
- When JD is empty, target role may be empty, but the server records that fact and does not fabricate a role.
- Generated ordinals must start at 1 and increase continuously.
- A failed generation must not write a partial question set.

Errors:

- `409 RESUME_NOT_PARSED`
- `404 RESUME_NOT_FOUND`
- `404 JOB_DESCRIPTION_NOT_FOUND`
- `503 QUESTION_GENERATION_UNAVAILABLE`

### BE-QSET-02 List

```http
GET /api/v1/question-sets
```

Query parameters:

| Parameter   | Allowed values                       |
| ----------- | ------------------------------------ |
| `status`    | `ready`, `archived`                  |
| `resume_id` | UUID                                 |
| `page`      | Positive integer                     |
| `page_size` | 1–100                               |
| `sort`      | `created_at_desc`, `created_at_asc` |

Response: `200 PageResponse<QuestionSetSummaryResponse>`.

### BE-QSET-03 Details

```http
GET /api/v1/question-sets/:id
```

Response: `200 QuestionSetDetailResponse`.

### BE-QSET-04 Update question set

```http
PATCH /api/v1/question-sets/:id
```

Request:

```ts
type QuestionInput = {
  ordinal: number;
  question: string;
  intent: string;
  expected_points: string[];
  follow_up_hint?: string;
};

type UpdateQuestionSetRequest = {
  target_role?: string;
  status?: QuestionSetStatus;
  questions?: QuestionInput[];
};
```

Response: `200 QuestionSetDetailResponse`.

Rules:

- Provide at least one field.
- Providing `questions` means replacing the complete question collection, not appending partially.
- Ordinals must be `1..N`, unique, and continuous.
- A question collection must contain at least 1 and at most 50 questions.
- Complete the update in one transaction.
- An existing interview stores a question snapshot, so editing a question set does not change questions in historical interviews.

### BE-QSET-05 Delete

```http
DELETE /api/v1/question-sets/:id
```

Response: `204 No Content`.

Rules:

- Return `409 QUESTION_SET_IN_USE` when related interviews exist; recommend archiving instead of deleting.
- Delete the question set and its questions when no relations exist.
- Do not cascade-delete interviews.
- If the resource does not exist, still return `204` for delete idempotency.

### BE-QSET-06 Regenerate

```http
POST /api/v1/question-sets/:id/regenerate
```

Request:

```ts
type RegenerateQuestionSetRequest = {
  job_description_id?: string;
  target_role?: string;
};
```

Response: `201 QuestionSetDetailResponse`.

Rules:

- Always create a new question set; do not overwrite the original.
- When override fields are absent, inherit the original question set’s resume, JD, and target role.
- Save `source_question_set_id` on the new record.
- Keep the original question set and its historical interviews unchanged.

### Question-set data and TODO

- [x] Add `updated_at` to `question_sets`.
- [x] Add nullable self-referencing foreign key `source_question_set_id` to `question_sets`.
- [x] Implement list, details, complete-replacement update, relationship-check deletion, and regeneration.
- [x] Add tests for ordinals, transaction rollback, historical interview snapshots, and multi-user isolation.

## 9. P0: Interview interfaces

### 9.1 Interview timing and state rules

When creating an interview:

- `status = active`.
- `started_at = now()`.
- `question_duration_seconds` defaults to `180`.
- The first question has `started_at = now()`; later questions are unstarted.

Question state is derived from persisted fields:

| Condition                                  | State       |
| ------------------------------------------ | ----------- |
| `started_at` is null                       | `unstarted` |
| Started, with empty `answer` and `skipped_at` | `answering` |
| `answer` is non-empty                      | `answered`  |
| Empty `answer` with non-empty `skipped_at` | `skipped`   |

Rules:

- Interview timing cannot be paused.
- A single-question timeout does not auto-submit or auto-skip.
- When an answer is overwritten, update `answered_at` and clear `skipped_at`.
- Set `started_at` for the next question the first time it is reached.
- After `completed`, all question writes return a conflict.
- The server is the sole authority for timing and completion state.

### BE-INTERVIEW-01 Create interview

```http
POST /api/v1/interviews
```

Request:

```ts
type CreateInterviewRequest = {
  resume_id: string;
  question_set_id: string;
  job_description_id?: string;
  title: string;
  question_duration_seconds?: number;
};
```

Response: `201 InterviewSessionResponse`.

Rules:

- `resume_id` must match the resume associated with the question set.
- An optional JD must belong to the current user.
- `question_duration_seconds` only allows `180` in Phase 1; retain the field for later configuration.
- Create the Session and question-snapshot Turns in the same transaction.
- Return `409 QUESTION_SET_EMPTY` when the question set contains no questions.

### BE-INTERVIEW-02 Interview list

```http
GET /api/v1/interviews
```

Query parameters:

| Parameter   | Allowed values                                      |
| ----------- | --------------------------------------------------- |
| `status`    | `draft`, `active`, `completed`, `abandoned`        |
| `page`      | Positive integer                                    |
| `page_size` | 1–100                                             |
| `sort`      | `updated_at_desc`, `updated_at_asc`, `created_at_desc` |

Response: `200 PageResponse<InterviewSummaryResponse>`.

The list must return question count, answered count, skipped count, and optional report overall score through one query or controlled batch queries; per-item N+1 queries are prohibited.

### BE-INTERVIEW-03 Interview details

```http
GET /api/v1/interviews/:id
```

Response: `200 InterviewSessionResponse`.

Rules:

- Return every field needed to restore both timers after refresh.
- `duration_seconds`:
  - Active: `now - started_at`.
  - Completed: `completed_at - started_at`.
- Sort Turns by ascending `ordinal`.

### BE-INTERVIEW-04 Save or overwrite an answer for a specified question

```http
PUT /api/v1/interviews/:id/turns/:ordinal/answer
```

Request:

```ts
type SaveInterviewAnswerRequest = {
  answer: string;
};
```

Response: `200 InterviewSessionResponse`.

Rules:

- Only an `active` interview can be written.
- Allow first answers and overwrites.
- Treat an all-whitespace answer as a validation error, not as a skip.
- Set `answered_at = now()` after saving and clear `skipped_at`.
- If the question has not started, set `started_at = now()` at the same time.
- Make the operation idempotent for the final request content.
- If the operation reaches the next unstarted question in the sequential flow, start it and update `current_ordinal`.
- Use transactions and row locks to prevent concurrent requests from producing an invalid state.

Errors:

- `404 INTERVIEW_NOT_FOUND`
- `404 INTERVIEW_TURN_NOT_FOUND`
- `409 INTERVIEW_ALREADY_COMPLETED`

### BE-INTERVIEW-05 Skip a specified question

```http
POST /api/v1/interviews/:id/turns/:ordinal/skip
```

Request: no Body.

Response: `200 InterviewSessionResponse`.

Rules:

- Only an `active` interview can be operated on.
- When there is no answer, record `skipped_at`; when an answer already exists, return `409 TURN_ALREADY_ANSWERED`. The frontend may retain the answer or overwrite it first.
- If the question has not started, set `started_at = now()` at the same time.
- Repeated skips of the same question remain idempotent and do not change the time again.
- Start the next unstarted question and update `current_ordinal`.

### BE-INTERVIEW-06 Complete the whole interview

```http
POST /api/v1/interviews/:id/complete
```

Request:

```ts
type CompleteInterviewRequest = {
  confirm_incomplete: boolean;
};
```

Response: `200 InterviewSessionResponse`.

Rules:

- When `unstarted` or `skipped` questions exist and `confirm_incomplete=false`, return `409 INTERVIEW_HAS_UNANSWERED_TURNS`.
- Return all unanswered ordinals in `details.ordinals`.
- After confirmation, set the interview to `completed` and write `completed_at` and `duration_seconds`.
- Lock all answers, skips, and repeated state mutations after completion.
- Repeated completion returns the current completed Session for idempotency.
- Completion need not generate a report synchronously; generate and reuse it on the first report request.

### BE-INTERVIEW-07 Migrate the legacy sequential-answer interface

```http
POST /api/v1/interviews/:id/answer
```

- [x] Keep the old interface during the new-interface rollout and mark it deprecated.
- [x] Reuse the new “save answer for current question” business logic internally.
- [!] The frontend has switched to the specified-question interface; schedule deletion after one compatible release cycle.
- [!] Before deletion, confirm that no caller or automated test depends on it.

### Interview data migration

Add to `interview_sessions`:

```text
started_at                  timestamptz
duration_seconds            integer NOT NULL DEFAULT 0
question_duration_seconds   integer NOT NULL DEFAULT 180
```

Add to `interview_turns`:

```text
started_at          timestamptz
skipped_at          timestamptz
time_spent_seconds  integer NOT NULL DEFAULT 0
```

Constraints:

- All duration/time_spent fields must not be less than 0.
- `question_duration_seconds` is limited to 180 in Phase 1.
- For existing `active` sessions, use `created_at` to backfill missing `started_at`.
- For existing `completed` sessions, use `completed_at - created_at` to backfill duration.
- For answered questions, use `created_at`/`answered_at` to backfill question start and time spent; when exact recovery is impossible, record the migration explanation and do not fabricate precise meaning.

### Interview TODO

- [x] Complete database migration and backfill.
- [x] Implement list, specified-question save, skip, and completion interfaces.
- [x] Upgrade detail response fields.
- [x] Use transactions and necessary row locks for all state changes.
- [x] Add tests for refresh recovery, concurrent saves, repeated requests, completion locking, and multi-user isolation.

## 10. P0: Report interfaces

### BE-REPORT-01 Get or generate report

```http
GET /api/v1/interviews/:id/report
```

Response: `200 InterviewReportResponse`.

Rules:

- Only a `completed` interview can generate a report.
- Generate and persist the report when it does not exist.
- Reuse an existing report on later requests; do not regenerate on every query.
- Concurrent first requests can write only one report successfully.
- Per-question reports must return the original question and the user’s answer.
- Unanswered questions may have an empty `answer`, but must still receive scoring and critique appropriate to the missing answer.
- `score` and `overall_score` range from 0 to 100.
- Return a retryable error when the report model or a dependency fails; do not write a partial report.

Errors:

- `404 INTERVIEW_NOT_FOUND`
- `409 INTERVIEW_NOT_COMPLETED`
- `503 REPORT_GENERATION_UNAVAILABLE`

### Report TODO

- [x] Add `status`, question, original answer, creation time, and update time to the response.
- [x] Preserve the constraint of exactly one report per interview.
- [x] Add tests for concurrent-generation reuse, unanswered questions, and the quality gate.
- [x] Do not develop server-side PDF in Phase 1; use browser printing/export.

## 11. P1: Task-center interfaces

### BE-TASK-01 Single task

```http
GET /api/v1/tasks/:id
```

Response: `200 TaskResponse`.

Upgrade requirements:

- Add related object, structured error, retry source, and start/end times to existing fields.
- `progress` must be between 0 and 100.
- Do not return raw Worker exceptions directly to the frontend for failed tasks.

### BE-TASK-02 Task list

```http
GET /api/v1/tasks
```

Query parameters:

| Parameter   | Allowed values                              |
| ----------- | ------------------------------------------- |
| `status`    | `pending`, `running`, `succeeded`, `failed` |
| `type`      | Task type, such as `resume.parse`           |
| `page`      | Positive integer                            |
| `page_size` | 1–100                                      |
| `sort`      | `created_at_desc`, `created_at_asc`        |

Response: `200 PageResponse<TaskResponse>`.

The formal product primarily returns `resume.parse` in Phase 1. Beta ASR tasks remain protected by user isolation; whether the formal task center displays them is determined by frontend product configuration.

### BE-TASK-03 Retry a failed task

```http
POST /api/v1/tasks/:id/retry
```

Response: `202 TaskAcceptedResponse`.

Rules:

- Allow only failed tasks whose type supports retry.
- Create a new Task ID and record `retry_of_task_id`.
- Keep the original task read-only; do not change it back to pending.
- If a pending/running retry already exists for the same failed task, return that task for idempotency.
- Return `409 TASK_NOT_RETRYABLE` for unsupported retries.

### Task data and TODO

- [x] Add nullable self-referencing foreign key `retry_of_task_id` to `async_tasks`.
- [x] Design structured, safe `error_code` and user-facing error-summary fields.
- [x] Implement list, filtering, pagination, and retry.
- [x] Make Worker updates for failure, success, and completion time consistent.
- [x] Add duplicate-retry and cross-user-access tests.

## 12. P1: Dashboard interfaces

### BE-DASH-01 Dashboard summary

```http
GET /api/v1/dashboard/summary
```

Response:

```ts
type DashboardSummaryResponse = {
  counts: {
    resumes: number;
    job_descriptions: number;
    question_sets: number;
    interviews: number;
    completed_interviews: number;
  };
  average_score?: number;
  score_trend: Array<{
    date: string; // YYYY-MM-DD，按 UTC 聚合
    average_score: number;
    interview_count: number;
  }>;
  improvement_topics: Array<{
    label: string;
    count: number;
  }>;
  recent_resumes: ResumeSummaryResponse[];
  recent_job_descriptions: JobDescriptionResponse[];
  recent_interviews: InterviewSummaryResponse[];
  active_tasks: TaskResponse[];
};
```

Rules:

- Omit `average_score` when there are no completed reports; do not return a fabricated 0.
- The default trend covers the most recent 30 days and counts only completed interviews with reports.
- Return at most 5 recent resources of each type.
- Active tasks include only `pending/running`, with at most 5 items.
- Improvement topics come from real report data, with at most 5 items; return `[]` when there is no data.
- Avoid N+1 queries and add necessary indexes for user, status, and time conditions.

### Dashboard TODO

- [x] Implement summary aggregation queries.
- [x] Define normalization for improvement topics so synonymous text is not split indefinitely.
- [x] Add tests for completely empty new users, partial data, and complete data.
- [x] Check execution plans and performance for aggregation queries.

## 13. Database migration summary

| Priority | Table | Change |
| -------- | ----- | ------ |
| P0 | New Refresh Session table | Formal refresh and logout sessions |
| P0 | `question_sets` | `updated_at`, `source_question_set_id` |
| P0 | `interview_sessions` | `started_at`, `duration_seconds`, `question_duration_seconds` |
| P0 | `interview_turns` | `started_at`, `skipped_at`, `time_spent_seconds` |
| P1 | `async_tasks` | `retry_of_task_id`, structured error fields |
| P1 | Report-related data | Add normalized improvement topics or a derived table if stable aggregation is needed |

Requirements:

- [x] Provide a Goose Up/Down migration for every change.
- [x] Up migrations can run against existing data and have explicit backfill rules.
- [x] New indexes consider `user_id + status + time` list queries.
- [x] Do not modify existing migration files; use new numbered incremental migrations.
- [x] Submit migration, sqlc queries, and application logic in the same change.

## 14. Interface implementation priority

### P0-A: Blocks the frontend foundation loop

- [x] Unify error responses and validation errors.
- [x] Upgrade resume/JD list pagination.
- [x] Upgrade resume detail fields.
- [x] Question-set list and details.
- [x] Interview detail timing fields.
- [x] Add questions and original answers to reports.

### P0-B: Blocks formal resource management

- [x] Resume rename, deletion, and reparsing.
- [x] JD details, update, and deletion.
- [x] Question-set update, deletion, and regeneration.

### P0-C: Blocks the complete interview experience

- [x] Interview list.
- [x] Save/overwrite answers for specified questions.
- [x] Skip specified questions.
- [x] Actively complete the whole interview.
- [x] Server-side timing and refresh recovery.

### P1: Product data loop

- [x] Dashboard summary.
- [x] Task list and retry.
- [x] Update profile and password.
- [x] Refresh Cookie and logout.

### P2: Not implemented in this round

- Password recovery and reset.
- Email verification.
- Account deletion.
- Interfaces required by the resume multi-version-management UI.
- Server-side PDF for reports.
- Formal productization of company interview intelligence and ASR.

## 15. Compatibility and release strategy

### 15.1 Upgrade list responses

Current resume and JD lists return arrays directly; the formal contract changes them to `PageResponse<T>`. The project has not been formally released, so upgrade `/api/v1` directly and regenerate the frontend SDK; if external callers already exist, you must:

1. Inventory callers.
2. Temporarily provide a compatibility adapter or new version path.
3. Remove the old response after callers migrate.

### 15.2 Migrate interview answers

The new frontend uniformly uses:

```http
PUT /interviews/:id/turns/:ordinal/answer
```

Keep the old `POST /interviews/:id/answer` for one release cycle and mark it deprecated. The old and new interfaces must reuse the same domain logic to avoid behavior divergence.

### 15.3 SDK generation

Every interface change must be checked in CI:

- `.api` can successfully generate Go, OpenAPI, and TypeScript.
- Generated files match repository contents with no uncommitted differences.
- Frontend `typecheck` passes.
- Backend and frontend contract tests pass.

## 16. Testing and acceptance

### 16.1 Unit and integration tests

- [x] Normal, validation-failure, unauthenticated, resource-not-found, and state-conflict cases for every interface.
- [x] Cross-user isolation for all resources.
- [x] List pagination boundaries, stable sorting, and empty lists.
- [x] Deletion conflicts and historical-data retention.
- [x] Upload completion, repeated completion, and missing objects.
- [x] Parse failures, reparsing, and duplicate retries.
- [x] Transaction rollback for complete question-set replacement.
- [x] Interview skipping, returning to edit, repeated completion, and post-completion locking.
- [x] Consistent timing fields after an interview refresh.
- [x] First report generation, repeated reuse, and concurrent reuse.
- [x] Refresh Token rotation, replay, and logout revocation.

### 16.2 Contract tests

- [x] HTTP status, response fields, and enums match `.api`.
- [x] `ErrorResponse` contains a stable `code` and request ID.
- [x] All times are valid UTC RFC 3339.
- [x] All empty collections return `[]`.
- [x] The SDK can complete the frontend core flow directly without hand-written supplementary DTOs.

### 16.3 Performance and reliability

- [x] List and dashboard queries have no N+1.
- [x] With default pagination, core list queries meet the project’s agreed latency target.
- [x] Actions such as deletion, interview completion, and task retry are idempotent or return clear conflicts.
- [x] Worker state is eventually persisted; failures do not remain in `running` forever.
- [x] Logs contain request ID, user ID, resource ID, and error code, but not passwords, Tokens, or complete resume text.

## 17. Final backend acceptance checklist

- [x] All P0/P1 interfaces required by the frontend are written into `.api`.
- [x] Go, OpenAPI, and the TypeScript SDK have been regenerated.
- [x] The complete “resume → JD → question set → interview → report” flow passes API automation tests.
- [x] Interviews support skipping, returning to overwrite, active completion, and timing recovery after refresh.
- [x] Completed interviews cannot be modified.
- [x] Resume, JD, and question-set management have explicit relationship rules.
- [x] Dashboard and task center return only real data for the current user.
- [x] Authentication supports short-lived Access Tokens, Refresh Cookies, and logout revocation.
- [x] Unified pagination, error, time, and status conventions are fully implemented.
- [x] All new migrations execute successfully against existing data.
- [x] Backend tests, generation checks, and project acceptance scripts all pass.
