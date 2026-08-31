# InterviewMaster Frontend Refactor TODO

> **Archived (2026-08-28).** The “to be developed” items in this document are outdated. Current tasks are in [`../invite-beta-todo_0828.md`](../invite-beta-todo_0828.md).
> Document status (when written): To be developed
> Updated: 2026-07-29
> Requirements baseline: [`frontend-refactor-requirements.md`](./frontend-refactor-requirements.md)
> Backend contract: [`backend-refactor-todo.md`](./backend-refactor-todo.md)
> Frontend directory: `web/`

## 1. Goals and boundaries

The goal of this round is to refactor the current single-page technical validation workbench into a complete Web product for real users, delivering the following core loop:

```text
Website → Register/Log in → Upload and parse resume → Add optional JD → Generate question set
        → Create mock interview → Answer one question at a time → Complete interview → View report
```

The formal product in this round does not include:

- Company interview-intelligence Beta.
- ASR speech transcription Beta.
- Real-time voice, video, or digital-human interviews.
- English UI, community, job applications, or recruiter back office.
- Password recovery, email verification, or account deletion.
- Complex long-term relationships between a resume and multiple JDs.

Do not create fake-success interactions for capabilities that the backend does not yet implement. The corresponding operation must be hidden or clearly disabled and linked to a backend TODO.

## 2. Development conventions

### 2.1 Status markers

- `[ ]` Not started.
- `[-]` In progress.
- `[x]` Completed and accepted.
- `[!]` Blocked by a backend interface or product decision.

### 2.2 Definition of done

An individual task may be marked complete only when all of the following are satisfied:

1. It works on both desktop and mobile.
2. Light, dark, and system-following modes work correctly.
3. Loading, empty, success, failure, and unauthorized states are complete.
4. Keyboard operation, focus states, and accessible form names are correct.
5. No fabricated business data is used, and raw backend errors are not exposed directly.
6. Relevant component or end-to-end tests pass.
7. `lint`, `typecheck`, unit tests, and the production build pass.

## 3. Route and page list

| ID          | Route                      | Page              | Core requirements                                      | Dependent interfaces             |
| ----------- | -------------------------- | ----------------- | ------------------------------------------------------ | -------------------------------- |
| FE-ROUTE-01 | `/`                        | Product website   | Chinese value proposition, three-step flow, capabilities, scenarios, CTA, footer | None                             |
| FE-ROUTE-02 | `/login`                   | Login             | Email/password, instant validation, error feedback, return to the original page | `POST /auth/login`               |
| FE-ROUTE-03 | `/register`                | Registration      | Display name, email, password, confirmation, automatic login after success | `POST /auth/register`            |
| FE-ROUTE-04 | `/dashboard`               | Dashboard         | New training, statistics, trends, recent resources, active tasks, empty state | `GET /dashboard/summary`         |
| FE-ROUTE-05 | `/resumes`                 | Resume list       | Status/time filters, rename, delete, reparse          | Resume list and management APIs  |
| FE-ROUTE-06 | `/resumes/new`             | Resume upload     | File validation, presigned upload, real progress, parse-task tracking | Resume upload and task APIs      |
| FE-ROUTE-07 | `/resumes/[id]`            | Resume details    | File information, version, structured facts, source excerpts, management actions | Resume detail and management APIs |
| FE-ROUTE-08 | `/jobs`                    | JD list           | Company, role, tags, time, view/edit/delete            | JD list and management APIs      |
| FE-ROUTE-09 | `/jobs/new`                | New JD            | Company, role, long text, character count, validation | `POST /job-descriptions`         |
| FE-ROUTE-10 | `/jobs/[id]`               | JD detail/edit    | View capability tags, edit, delete                    | JD detail and management APIs    |
| FE-ROUTE-11 | `/question-sets`           | Question-set list | Filter, summary, view, delete, regenerate              | Question-set list and management APIs |
| FE-ROUTE-12 | `/question-sets/new`       | Generate question set | Resume required, JD/role optional, material summary, generation status | `POST /question-sets`            |
| FE-ROUTE-13 | `/question-sets/[id]`      | Question-set detail | Question preview, intent, answer points, follow-ups, edit, start interview | Question-set detail and management APIs |
| FE-ROUTE-14 | `/interviews`              | Interview history | In progress/completed, progress, score, filters, continue/view | `GET /interviews`                |
| FE-ROUTE-15 | `/interviews/new`          | Create interview  | Select question set, automatic summary, title, timing explanation | Question-set list, create-interview |
| FE-ROUTE-16 | `/interviews/[id]`         | Interview room    | One question, dual timers, skip, go back, edit answer, submit, resume | Interview detail and control APIs |
| FE-ROUTE-17 | `/interviews/[id]/report`  | Interview report  | Overall score, charts, per-question analysis, print/PDF | `GET /interviews/:id/report`     |
| FE-ROUTE-18 | `/tasks`                   | Task center       | Type, object, status, progress, filters, retry failures | Task list and retry APIs         |
| FE-ROUTE-19 | `/settings`               | Settings          | Profile, theme, change password, log out              | User and authentication APIs     |

## 4. P0: Engineering foundations

### FE-BASE-01 Initialize dependencies and directories

- [ ] Add Tailwind CSS.
- [ ] Add shadcn/ui and keep only components that are actually used.
- [ ] Add TanStack Query.
- [ ] Add React Hook Form and Zod.
- [ ] Add `next-themes`.
- [ ] Add Testing Library and Playwright.
- [ ] Create the following directories:

```text
web/src/
  app/
    (marketing)/
    (auth)/
    (product)/
  components/
    ui/
    layout/
    feedback/
  features/
    auth/
    dashboard/
    resumes/
    jobs/
    question-sets/
    interviews/
    reports/
    tasks/
    settings/
  shared/
    api/
    config/
    hooks/
    lib/
    types/
  styles/
```

Acceptance:

- Page files only handle routing and module orchestration; they do not contain large blocks of API, form, or business-state logic.
- Generic non-business components live in `components/`; business components live in the corresponding `features/` directory.
- The website build output does not load post-login business modules.

### FE-BASE-02 Unified API Client

- [ ] Build on goctl-generated code in `web/src/shared/api/generated/`.
- [ ] Add a business API Client around the generated SDK; pages must not call `fetch` directly.
- [ ] Automatically inject `Authorization: Bearer <access_token>`.
- [ ] Normalize backend `ErrorResponse` consistently.
- [ ] Support `AbortSignal`, request timeouts, and upload progress.
- [ ] Save and display `request_id` from responses for troubleshooting.
- [ ] Handle 401 consistently: try to refresh the session; redirect to login on failure.
- [ ] Carry a safe internal `return_to` through login and return to the original page after re-authentication.
- [ ] Define Query Keys, cache times, invalidation rules, and polling strategies consistently.
- [ ] Do not hand-write duplicate server-response types in pages or features.

Normalized frontend API error:

```ts
type AppError = {
  code: string;
  message: string;
  requestId?: string;
  fieldErrors?: Record<string, string[]>;
  details?: unknown;
};
```

### FE-BASE-03 Query and form conventions

- [ ] TanStack Query manages all API data, caching, polling, and request state.
- [ ] Use React Hook Form for forms.
- [ ] Use Zod for input validation, with Simplified Chinese error messages.
- [ ] Use component state or a reducer for short-lived page UI state.
- [ ] Do not introduce Redux/Zustand in Phase 1.
- [ ] The server is authoritative for interview state; locally store only unsubmitted drafts and the currently displayed question number.

### FE-BASE-04 Authentication and route protection

- [ ] Unauthenticated users can access only the website, login, and registration.
- [ ] Authenticated users visiting `/login` or `/register` are redirected to `/dashboard`.
- [ ] Successful login defaults to `/dashboard`; if `return_to` exists, return there first.
- [ ] Neither server nor client may briefly render unauthorized product content.
- [ ] Use the Access Token only for short-lived request authentication; after the backend session upgrade, use an HttpOnly Refresh Cookie.
- [ ] Clear client caches, drafts, and user-related in-memory state after logout.

## 5. P0: Design system and product shell

### FE-UI-01 Theme and Design Tokens

- [ ] Define background, surface, text, border, brand, success, warning, and danger colors with CSS Variables.
- [ ] Support light, dark, and system-following modes.
- [ ] Default to system-following and persist the user’s choice.
- [ ] Avoid theme flashing during SSR/first paint.
- [ ] Do not scatter the hard-coded colors specified in the requirements document across components.
- [ ] Respect `prefers-reduced-motion`.

### FE-UI-02 Basic components

- [ ] Button, IconButton, Input, Textarea, Select, Checkbox.
- [ ] FormField, FieldError, PasswordInput, CharacterCounter.
- [ ] Card, Badge, Progress, Skeleton, Tooltip.
- [ ] Alert, Toast, EmptyState, ErrorState, OfflineBanner.
- [ ] Dialog, ConfirmDialog, Drawer, DropdownMenu.
- [ ] Pagination, FilterBar, ResponsiveList.
- [ ] Components have clear focus rings, disabled states, loading states, and accessible names.

### FE-UI-03 Responsive product layout

- [ ] Desktop left navigation: Dashboard, Resumes, JD, Question Sets, Mock Interviews, Task Center, Settings.
- [ ] Mobile top bar plus bottom navigation: Home, Resumes, Interviews, Me.
- [ ] Enter JD, Question Sets, and Task Center through home shortcuts or “Me”.
- [ ] Minimum width 320px with no horizontal scrolling.
- [ ] Touch targets at least 44×44px.
- [ ] Fixed bottom action areas respect safe areas.
- [ ] Convert mobile tables into cards or summary lists.
- [ ] Convert mobile dialogs into drawers or full-screen pages.

## 6. P0: Website and authentication

### FE-AUTH-01 Product website

- [ ] Header brand, capabilities, usage flow, login, and “Start for free” entry.
- [ ] Hero copy: “Complete a reviewable AI mock interview based on your resume and target role.”
- [ ] Three-step flow: Prepare materials, start interview, get report.
- [ ] Four core capabilities: resume understanding, tailored question sets, one-question-at-a-time interviews, review reports.
- [ ] Scenarios for new graduates and experienced job seekers.
- [ ] Bottom CTA and basic footer.
- [ ] On mobile, the hero value proposition and primary CTA are visible without a long scroll.
- [ ] Configure Chinese metadata, basic Open Graph information, and a reasonable semantic SEO structure.

### FE-AUTH-02 Login

- [ ] Email, password, and password-visibility toggle.
- [ ] Immediate email-format and required-field validation.
- [ ] In-button submission state to prevent duplicate submissions.
- [ ] Use unified user-facing copy for credential errors without revealing whether an account exists.
- [ ] Do not show unavailable third-party-login or password-recovery entries.

Dependency:

- `POST /api/v1/auth/login`

### FE-AUTH-03 Registration

- [ ] Display name, email, password, and password confirmation.
- [ ] Password rules match the backend.
- [ ] Confirm password is validated only on the frontend and is not sent to the backend.
- [ ] Create a session automatically after successful registration and enter the dashboard.
- [ ] When an email is already registered, focus the email field.

Dependency:

- `POST /api/v1/auth/register`

## 7. P0: Resumes

### FE-RESUME-01 Resume list

- [ ] Show title, filename, update time, and parsing status.
- [ ] Support status filtering, update-time sorting, and pagination.
- [ ] Use a list/table on desktop and vertical cards on mobile.
- [ ] The new-user empty state explains why a resume is needed and provides the primary upload action.
- [ ] `pending` and `processing` states show an entry point to task progress.
- [ ] `failed` shows an understandable reason and reparse action.
- [ ] Use a lightweight form for renaming.
- [ ] Show relationship impact and ask for confirmation before deletion.

Dependencies:

- `GET /api/v1/resumes`
- `PATCH /api/v1/resumes/:id`
- `DELETE /api/v1/resumes/:id`
- `POST /api/v1/resumes/:id/reparse`

### FE-RESUME-02 Upload and parsing

- [ ] Allow only PDF, DOCX, and TXT.
- [ ] File size must be greater than 0 and no more than 20 MiB.
- [ ] Request an upload credential, then `PUT` the file directly to the presigned URL.
- [ ] Use backend-returned `upload_headers` and show real upload percentage.
- [ ] Notify the backend to start parsing after upload completes.
- [ ] Poll task status using the returned `task_id`.
- [ ] Poll every 2 seconds while processing; pause active polling when the page is hidden.
- [ ] Invalidate the resume-list cache and navigate to details after success.
- [ ] After failure, show the summary, request ID, and reparse entry point.
- [ ] Warn that upload will be interrupted when leaving during upload.

Dependencies:

- `POST /api/v1/resumes/uploads`
- `PUT` to the presigned URL
- `POST /api/v1/resumes/:id/versions/:versionId/complete`
- `GET /api/v1/tasks/:id`

### FE-RESUME-03 Resume details

- [ ] Show title, original filename, version number, type, size, upload time, and parsing status.
- [ ] Display facts by categories such as personal information, education, employment, projects, and skills.
- [ ] Show a source excerpt for each fact; confidence is auxiliary information and must not be presented as an accuracy conclusion.
- [ ] Support rename, reparse, and delete.
- [ ] Show task status or a refresh entry while parsing.
- [ ] Show a safely processed failure reason when parsing fails.

Dependencies:

- `GET /api/v1/resumes/:id`
- `PATCH /api/v1/resumes/:id`
- `DELETE /api/v1/resumes/:id`
- `POST /api/v1/resumes/:id/reparse`

## 8. P0: JD

### FE-JOB-01 JD list

- [ ] Show company, role, capability tags, and update time.
- [ ] Support pagination, time sorting, viewing, editing, and deletion.
- [ ] Do not show an empty placeholder column when company is empty.
- [ ] Use cards on mobile.
- [ ] The empty state guides users to create a JD and explains that a JD is optional material.

### FE-JOB-02 Create and edit

- [ ] Fields: company (optional), role title, JD body.
- [ ] The body is suitable for pasting long text and shows a character count.
- [ ] Frontend and backend use the same length limits.
- [ ] Navigate to details after saving and show extracted capability tags.
- [ ] When content or role changes, explain that capability tags will be extracted again.
- [ ] Before deletion, explain that generated questions and historical interviews will not be rewritten.

Dependencies:

- `POST /api/v1/job-descriptions`
- `GET /api/v1/job-descriptions`
- `GET /api/v1/job-descriptions/:id`
- `PATCH /api/v1/job-descriptions/:id`
- `DELETE /api/v1/job-descriptions/:id`

## 9. P0: Question sets

### FE-QSET-01 Generate question set

- [ ] A resume is required, and only resumes with `completed` status can be selected.
- [ ] JD is optional.
- [ ] Target role is optional; prompt the user to fill it in when no JD is selected.
- [ ] Show a summary of the resume, JD, and target role before submission.
- [ ] Show clear status and an explanation that the user may leave during generation.
- [ ] Prevent duplicate submissions.
- [ ] Invalidate the question-set list and navigate to details after success.
- [ ] Preserve the user’s selections and allow retry after failure.

### FE-QSET-02 Question-set list and details

- [ ] The list shows target role, associated resume, optional JD, question count, status, and creation time.
- [ ] Details show questions, assessment intent, expected answer points, and follow-up hints in order.
- [ ] Support editing question text, intent, answer points, follow-up, and order.
- [ ] Validate the local structure when saving the complete question collection.
- [ ] Support deletion and regeneration.
- [ ] Make it clear that regeneration creates a new question set and leaves the original set and historical interviews unchanged.
- [ ] “Start mock interview” is the only primary action on the details page.

Dependencies:

- `POST /api/v1/question-sets`
- `GET /api/v1/question-sets`
- `GET /api/v1/question-sets/:id`
- `PATCH /api/v1/question-sets/:id`
- `DELETE /api/v1/question-sets/:id`
- `POST /api/v1/question-sets/:id/regenerate`

## 10. P0: Interviews

### FE-INTERVIEW-01 Interview history and creation

- [ ] Separate in-progress and completed interviews on the history page.
- [ ] Show title, associated question set, creation time, answer progress, status, and optional overall score.
- [ ] Support status filtering, time sorting, and pagination.
- [ ] In-progress interviews can be continued; completed interviews are view-only and can open the report.
- [ ] The create page selects a question set and automatically displays the resume, JD, and question count.
- [ ] Allow the interview title to be edited.
- [ ] Before starting, explain the default three minutes per question, the reference duration for the whole interview, and the non-pausable rule.
- [ ] Enter the interview room immediately after creation.

### FE-INTERVIEW-02 Interview room

- [ ] Focus mode; show only one question at a time.
- [ ] The header shows an exit entry, remaining time for the current question, elapsed time for the whole interview, and overall progress.
- [ ] The main area shows the question and necessary answer hints.
- [ ] Use a multiline text input and persist a local draft while typing.
- [ ] Bottom actions: Previous, Skip, Next/Save Answer, Complete Interview.
- [ ] A desktop sidebar or mobile drawer shows question navigation and status.
- [ ] Status copy: Not started, Answering, Answered, Skipped.
- [ ] Primary submit actions remain visible when the mobile keyboard is open.

Timing rules:

- [ ] Default countdown for the current question is 180 seconds.
- [ ] Reference duration for the whole interview is question count × 180 seconds.
- [ ] At 30 seconds remaining, show an icon/text warning as well as a color warning.
- [ ] A single-question timeout only shows timeout status; it does not submit or advance automatically.
- [ ] The whole interview cannot be paused after it starts.
- [ ] After refresh, restore from server `started_at` rather than resetting on the client.
- [ ] Timing calculations cover system-clock changes and recovery after the page returns from the background.

Answering rules:

- [ ] Save through the specified-question endpoint and allow overwriting an old answer.
- [ ] After skipping, move to the next question; return to answer before completing the whole interview.
- [ ] Editing an answered question overwrites the old answer.
- [ ] Do not show editable input controls for a completed interview.
- [ ] Require confirmation before completing when unanswered or skipped questions exist.
- [ ] When the backend returns an incomplete-question conflict, focus and mark the corresponding question numbers.

Leaving and offline behavior:

- [ ] Warn about unsaved text when navigating within the site or closing the page.
- [ ] Restore saved answers and timing through the server.
- [ ] Show a global offline notice and retain a local draft when disconnected.
- [ ] After connectivity returns, do not automatically overwrite the server answer; ask the user to confirm saving.
- [ ] Draft keys must include at least user ID, interview ID, and question number.
- [ ] Clear the corresponding draft after successful save, interview completion, or logout.

Dependencies:

- `GET /api/v1/interviews`
- `POST /api/v1/interviews`
- `GET /api/v1/interviews/:id`
- `PUT /api/v1/interviews/:id/turns/:ordinal/answer`
- `POST /api/v1/interviews/:id/turns/:ordinal/skip`
- `POST /api/v1/interviews/:id/complete`

Compatibility note:

- The legacy `POST /api/v1/interviews/:id/answer` supports only sequential first-time answers.
- After the new interview room migration is complete, it must no longer depend on the legacy endpoint.

## 11. P0: Reports

### FE-REPORT-01 Report display

- [ ] Show overall score, completion status, and report generation time.
- [ ] Use an accessible horizontal bar chart for per-question scores.
- [ ] Show strengths, priority improvement areas, and next-step training suggestions.
- [ ] For each question show the original question, user answer, score, critique, reference answer, and evidence.
- [ ] Express chart values with numbers and text in addition to color.
- [ ] Request/generate the report when it does not exist; allow retry after failure.
- [ ] Clearly explain when the interview is incomplete and provide an entry back to the interview.
- [ ] Use a single-column layout on mobile.

### FE-REPORT-02 Print and PDF

- [ ] Provide browser printing.
- [ ] Print styles hide navigation, buttons, Toasts, and other non-report content.
- [ ] Print pagination avoids unreasonable breaks between headings and body text or questions and analyses.
- [ ] In Phase 1, export through the browser’s “Save as PDF”; do not depend on server-side PDF generation.

Dependency:

- `GET /api/v1/interviews/:id/report`

## 12. P1: Dashboard, tasks, and settings

### FE-DASH-01 Dashboard

- [ ] “Start new training” is the primary action.
- [ ] Show recent resumes, JDs, interviews, and in-progress interviews.
- [ ] Show training count, nullable average score, and recent trend.
- [ ] Show frequent improvement areas.
- [ ] Show running or failed tasks.
- [ ] Show a real empty state when data is insufficient; do not fill in fabricated numbers.
- [ ] Show a first-use guide that starts with uploading a resume for new users.

### FE-TASK-01 Task center

- [ ] Show task type, related object, status, progress, and start/end times.
- [ ] Statuses: Pending, Processing, Succeeded, Failed.
- [ ] Support status and task-type filters and pagination.
- [ ] Show a safe error summary and retry entry for failed tasks.
- [ ] The formal navigation in Phase 1 shows resume-parsing tasks only.
- [ ] Reserve the structure for other task types but do not show a Beta ASR entry.

### FE-SETTINGS-01 Settings

- [ ] Profile: display name and read-only email.
- [ ] Appearance: light, dark, system-following.
- [ ] Security: current password, new password, confirmation of new password.
- [ ] Account: log out.
- [ ] Do not show an unimplemented account-deletion entry.

Dependencies:

- `GET /api/v1/dashboard/summary`
- `GET /api/v1/tasks`
- `POST /api/v1/tasks/:id/retry`
- `GET /api/v1/me`
- `PATCH /api/v1/me`
- `POST /api/v1/auth/change-password`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`

## 13. API usage rules

### 13.1 Time and enums

- All server times are UTC RFC 3339 strings.
- The frontend displays them in the user’s local time zone and does not assemble timezone-free timestamps when submitting.
- The frontend uses generated string-union types for statuses and provides a safe fallback for unknown statuses.

Key statuses:

```ts
type ResumeStatus =
  "draft" | "uploading" | "pending" | "processing" | "completed" | "failed";

type TaskStatus = "pending" | "running" | "succeeded" | "failed";
type QuestionSetStatus = "ready" | "archived";
type InterviewStatus = "draft" | "active" | "completed" | "abandoned";
type InterviewTurnState = "unstarted" | "answering" | "answered" | "skipped";
```

### 13.2 Pagination

All list endpoints consume:

```ts
type PageResponse<T> = {
  items: T[];
  page: number;
  page_size: number;
  total: number;
};
```

Rules:

- Pages start at 1.
- `page_size` defaults to 20 and has a maximum of 100.
- Empty lists return `items: []`, not `null`.
- Filters and sorting go into URL Search Params so they survive refresh and sharing.

### 13.3 Error handling

Backend error format:

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

Frontend behavior:

| HTTP status | Frontend behavior                                      |
| ----------- | ------------------------------------------------------ |
| 400         | Show field-level or request-level validation errors    |
| 401         | Try to refresh the session; redirect to login on failure |
| 403         | Show unauthorized state without exposing other users’ resources |
| 404         | Show resource-not-found state                          |
| 409         | Show the business conflict and impact                  |
| 429         | Show that requests are too frequent and suggest retrying later |
| 500/503     | Show a generic service error, request ID, and retry     |

### 13.4 Interface availability matrix

| Capability                 | Interface                                  | Current status | Frontend handling                         |
| -------------------------- | ------------------------------------------ | -------------- | ------------------------------------------ |
| Register/login/current user | `/auth/register`, `/auth/login`, `/me`    | Available      | Can be productized directly                |
| Resume upload/list/details  | `/resumes...`                              | Basic version available | List needs pagination; details need file fields |
| Resume rename/delete/reparse | `/resumes/:id...`                        | To be developed | Mark feature as blocked                    |
| JD create/list              | `/job-descriptions`                        | Basic version available | List needs pagination                       |
| JD details/update/delete    | `/job-descriptions/:id`                    | To be developed | Mark feature as blocked                    |
| Question-set generation     | `POST /question-sets`                      | Available      | Can be productized directly                |
| Question-set management     | `/question-sets...`                        | To be developed | Mark feature as blocked                    |
| Create/get interview        | `/interviews`, `/interviews/:id`           | Basic version available | Add timing and summary fields              |
| Sequential answer           | `POST /interviews/:id/answer`              | Legacy version available | Compatibility only; insufficient for formal UX |
| Skip/edit/complete          | `/interviews/:id/turns...`, `/complete`    | To be developed | Blocks complete interview interaction      |
| Report                      | `/interviews/:id/report`                   | Basic version available | Add question, original-answer, and time fields |
| Single task                 | `/tasks/:id`                               | Basic version available | Can poll upload parsing                    |
| Task list/retry             | `/tasks`, `/tasks/:id/retry`               | To be developed | Blocks task center                         |
| Dashboard                   | `/dashboard/summary`                       | To be developed | Blocks dashboard statistics                |
| Session refresh/logout      | `/auth/refresh`, `/auth/logout`            | To be developed | Blocks formal session management           |

## 14. Testing TODO

### FE-TEST-01 Unit and component tests

- [ ] Login, registration, JD, and settings form validation.
- [ ] API error normalization and field-error mapping.
- [ ] Query Keys and cache invalidation.
- [ ] Theme switching and no flash on first load.
- [ ] Single-question and whole-interview timing calculations.
- [ ] Skip, edit, complete, and locked-state transitions.
- [ ] Local-draft isolation, recovery, and cleanup.
- [ ] Report fields, empty answers, and evidence display.

### FE-TEST-02 End-to-end tests

- [ ] Register and enter the dashboard.
- [ ] Failed login, successful login, and safe return to the original page.
- [ ] Upload a resume and see parsing results.
- [ ] Create, edit, and delete a JD.
- [ ] Generate, edit, and regenerate a question set.
- [ ] Create an interview, skip, return to answer, overwrite an answer, and complete it.
- [ ] Refresh the interview page and restore answers, progress, and timing.
- [ ] Confirm completion when unanswered questions exist.
- [ ] Prevent answer changes after completion.
- [ ] View and print the report.
- [ ] Complete the core flow at 320px mobile width.
- [ ] Refresh or re-authenticate after Access Token expiry and return to the original page.

### FE-TEST-03 Visual, performance, and accessibility

- [ ] Screenshot baselines for the website, dashboard, resume list, interview room, and report page.
- [ ] Every baseline covers desktop/mobile and light/dark modes.
- [ ] Website Lighthouse desktop and mobile performance scores are at least 85.
- [ ] Core-page Lighthouse accessibility scores are at least 90.
- [ ] Complete login, upload, JD creation, interview, and report viewing by keyboard.
- [ ] No obvious layout shift; load large reports and long lists on demand.

## 15. Staged delivery

### Phase 1: Foundations and product shell

- [ ] All FE-BASE tasks.
- [ ] All FE-UI tasks.
- [ ] Website, login, and registration.
- [ ] Corresponding unit tests and visual baselines.

### Phase 2: Productize existing capabilities

- [ ] Resume upload, list, and details.
- [ ] JD creation and list.
- [ ] Question-set generation.
- [ ] Basic interview and report.
- [ ] Single-task status display.

### Phase 3: Complete resource and interview interaction

- [ ] Full resume, JD, and question-set management.
- [ ] Interview history.
- [ ] Skip, return, edit answer, and actively complete.
- [ ] Server-side timing recovery and offline drafts.

### Phase 4: Data loop and quality

- [ ] Dashboard.
- [ ] Task center.
- [ ] Settings and formal session management.
- [ ] Full E2E, visual regression, performance, and accessibility acceptance.

## 16. Final acceptance checklist

- [ ] Unauthenticated access to `/` shows a complete Chinese product website.
- [ ] All 19 confirmed pages have independent routes.
- [ ] Registration, login, session refresh, logout, and authentication redirects are complete.
- [ ] Users can complete the “resume → JD → question set → interview → report” loop.
- [ ] The interview shows one question at a time and both single-question and whole-interview timers.
- [ ] Before completion, users can skip, return, and overwrite answers; after completion, everything is locked.
- [ ] All resources and asynchronous operations have complete state feedback.
- [ ] Both light and dark themes pass visual acceptance.
- [ ] There is no horizontal scrolling at 320px, and the core flow works on mobile.
- [ ] Beta interview-intelligence and ASR formal entries are not shown.
- [ ] TypeScript has no unexplained `any`.
- [ ] `pnpm lint`, `pnpm typecheck`, `pnpm test`, and `pnpm build` all pass.
- [ ] Core pages pass keyboard-operation and basic accessibility checks.
