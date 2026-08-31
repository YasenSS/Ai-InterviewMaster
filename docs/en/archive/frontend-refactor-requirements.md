# InterviewMaster Frontend Refactor Requirements Specification

> **Archived (2026-08-28).** Requirements from 07-29; the main flow includes JD/question sets and must not be treated as the current specification.
> Document status (when written): Confirmed; usable for solution design and development breakdown
> Updated: 2026-07-29
> Scope: InterviewMaster Web frontend and the backend interface additions it depends on

## 1. Document purpose

The current frontend has one centralized workbench page that can validate the main backend flow, but it has not reached formal-product standards in information architecture, visual experience, interaction completeness, mobile adaptation, or engineering maintainability.

This refactor upgrades the existing validation page into a complete Web product for real users. This document is the unified requirements baseline for subsequent UI design, frontend development, backend interface additions, and acceptance.

This phase produces only the refactor specification; it does not directly modify the existing frontend or backend implementation.

## 2. Confirmed product decisions

| Item | Decision |
| --- | --- |
| Product name | InterviewMaster |
| Chinese positioning | AI Interview Training Assistant |
| Core goal | Visual upgrade and complete productization |
| Core users | New graduates and experienced job seekers |
| Interface language | Simplified Chinese |
| Primary devices | Desktop and mobile |
| Home page | Unauthenticated users see the product website |
| Post-login home | Product dashboard |
| Core flow | Register/log in → upload resume → add JD → generate question set → mock interview → view report |
| Visual style | Simple, professional, ChatGPT-like |
| Theme | Support both light and dark |
| Interview format | Conversational interface showing one question at a time |
| Timing | Per-question timer + whole-interview timer |
| Beta features | Company interview intelligence and speech transcription do not enter the formal product in this round |
| Resume and JD relationship | Product rules pending; Version 1 implements “resume required, JD optional” |
| New capabilities | Features not yet implemented by the current backend may be proposed and recorded uniformly as backend TODOs |

## 3. Product goals

### 3.1 User goals

- New users can quickly understand the product value and register.
- Users can clearly upload, parse, and view a resume.
- Users can save job descriptions and generate interview questions from a resume and optional JD.
- Users can complete a mock interview one question at a time in an interface with a realistic rhythm.
- Users can understand strengths, issues, and next training directions through a structured report.
- Users can complete the core flow smoothly on both desktop and mobile.

### 3.2 Business goals

- Upgrade the project from a technical demo page into an iteratable product interface.
- Establish a clear page system, design system, and component boundaries.
- Leave room for future voice interviews, company interview intelligence, and more AI capabilities.
- Make frontend/backend interface gaps traceable and deliverable in stages.

### 3.3 Engineering goals

- Keep React as the core of frontend development.
- Use Next.js App Router for the website, product routes, and rendering capabilities.
- Unify API calls, server state, form validation, error feedback, and loading states.
- Split the current single-file page into testable and reusable business modules.
- Continue using `backend/api/interviewmaster.api` and its generated types as the API contract baseline.

## 4. Non-goals

The following are outside the first-stage refactor:

- Formal productization of the company interview-intelligence Beta.
- Formal productization of the ASR speech-transcription Beta.
- Real-time voice interviews, video interviews, or digital humans.
- Community, job applications, or recruiter back office.
- English UI; the engineering structure may reserve room for internationalization, but the first release provides Chinese only.
- Complex version relationships between one resume and multiple JDs.
- Native mobile applications.

## 5. Product principles

1. **Emphasize one primary action at a time**: every page should have a clear primary task and primary button.
2. **Present complex capabilities step by step**: upload, generation, and interview flows use staged interactions rather than putting everything on one page.
3. **Keep state visible**: parsing, generating, completed, and failed states must be clearly displayed.
4. **Make results explainable**: reports show not only scores, but also evidence, issues, and actionable suggestions.
5. **Mobile is not a shrunken desktop**: rearrange navigation, operation areas, and content density for mobile.
6. **Avoid fake capabilities**: unsupported backend operations must not look like successful frontend interactions.

## 6. Information architecture and page map

### 6.1 Public area

| Route | Page | Description |
| --- | --- | --- |
| `/` | Product website | Product value, core flow, feature introduction, and registration entry |
| `/login` | Login | Email and password login |
| `/register` | Registration | Registration with email, password, and display name |

After successful login, redirect to `/dashboard` by default. Authenticated users visiting `/login` or `/register` are automatically redirected to the dashboard.

### 6.2 Post-login product area

| Route | Page | Description |
| --- | --- | --- |
| `/dashboard` | Dashboard | New-training entry, recent content, training data, and task status |
| `/resumes` | Resume list | Manage resumes and parsing status |
| `/resumes/new` | Resume upload | Create an upload credential, upload a file, and track parsing |
| `/resumes/[id]` | Resume details | Show file information and structured facts extracted from parsing |
| `/jobs` | JD list | Manage job descriptions |
| `/jobs/new` | New JD | Enter company, role, and job description |
| `/jobs/[id]` | JD details/edit | View capability tags, edit, or delete a JD |
| `/question-sets` | Question-set list | View and manage generated question sets |
| `/question-sets/new` | Generate question set | Select a resume, optional JD, and target role |
| `/question-sets/[id]` | Question-set details | Preview and edit questions and start an interview |
| `/interviews` | Interview history | View in-progress and completed interviews |
| `/interviews/new` | Create interview | Select a question set and configure the interview |
| `/interviews/[id]` | Interview room | One-question dialogue, dual timers, skip, and re-answer |
| `/interviews/[id]/report` | Interview report | Overall score, per-question analysis, and improvement suggestions |
| `/tasks` | Task center | View asynchronous tasks such as resume parsing |
| `/settings` | Settings | Profile, theme, security, and account settings |

### 6.3 Navigation structure

Desktop uses a left navigation bar:

- Dashboard
- Resumes
- JD
- Question Sets
- Mock Interviews
- Task Center
- Settings

Mobile uses a top bar and bottom primary navigation:

- Home
- Resumes
- Interviews
- Me

Enter JD, Question Sets, and Task Center through home shortcuts or the “Me” menu to avoid overcrowding the bottom navigation.

## 7. Core user flows

### 7.1 First use

1. The user visits the website and learns the product value.
2. The user opens registration and enters a display name, email, and password.
3. After registration, the user is logged in automatically and enters the dashboard.
4. The first-use flow guides the user to upload a resume.
5. During parsing, show task progress and product-feature guidance.
6. After parsing completes, guide the user to add a JD or enter a target role directly.
7. The user generates a question set and previews the questions.
8. The user creates a mock interview and answers one question at a time.
9. The user submits the whole interview and views the report.

### 7.2 Returning users

1. After login, enter the dashboard.
2. Continue an unfinished interview, or create new training from an existing resume/JD.
3. View previous reports in interview history.
4. Start the next round of training based on improvement suggestions.

### 7.3 Resume upload and parsing

1. The user selects a PDF, DOCX, or TXT file; the frontend validates file type and the 20 MB size limit.
2. The frontend requests an upload credential.
3. The frontend `PUT`s the file directly to the presigned URL and shows upload progress.
4. After upload completes, notify the backend to start parsing.
5. The frontend creates a task-tracking state.
6. Navigate to resume details after successful parsing; show an understandable reason and retry entry on failure.

## 8. Visual and design system

### 8.1 Brand direction

- Product name: InterviewMaster.
- Chinese subtitle: AI Interview Training Assistant.
- Brand character: trustworthy, restrained, professional, and friendly.
- Do not use excessive glow, complex gradients, or game-like visuals.
- Use green only for primary actions, success states, and limited brand recognition; do not cover large areas with it.

### 8.2 Suggested colors

Light theme:

- Page background: `#F7F7F8`
- Primary surface: `#FFFFFF`
- Primary text: `#171717`
- Secondary text: `#6B7280`
- Border: `#E5E7EB`
- Brand color: `#10A37F`
- Danger color: `#DC2626`
- Warning color: `#D97706`

Dark theme:

- Page background: `#171717`
- Primary surface: `#212121`
- Secondary surface: `#2F2F2F`
- Primary text: `#ECECEC`
- Secondary text: `#A3A3A3`
- Border: `#3F3F46`
- Brand color: `#19C37D`

Final colors must be managed through CSS Variables; hard-coded color values must not be scattered through components.

### 8.3 Typography

- Prefer system Chinese sans-serif fonts.
- Default body text is 14–16 px.
- Body line height is at least 1.6.
- Use no more than three visual levels for page titles, section titles, and body text.
- Interview question text should use a larger font and comfortable line length.

### 8.4 Radius and shadows

- Use 8–10 px radius for form controls and buttons.
- Use 12–16 px radius for cards.
- Rely primarily on backgrounds and borders for hierarchy; keep shadows light.
- Dialogs and overlays may use more visible but soft shadows.

### 8.5 Theme switching

- Support light, dark, and system-following choices.
- Default to system-following.
- Persist the user’s choice locally.
- Switching themes must not flash or lose page state.

## 9. Page-level requirements

### 9.1 Product website

The website should include:

- Header brand, capabilities, usage flow, login, and free-start entry.
- Hero value proposition: “Complete a reviewable AI mock interview based on your resume and target role.”
- Three-step flow: Prepare materials, start interview, get report.
- Core capabilities: resume understanding, tailored question sets, one-question-at-a-time interview, and review report.
- Usage scenarios for new graduates and experienced job seekers.
- Bottom CTA and basic footer.

On mobile, the hero must show the core value and primary CTA without excessive scrolling.

### 9.2 Login and registration

- Login fields: email and password.
- Registration fields: display name, email, password, and password confirmation.
- Forms provide immediate validation, password visibility switching, and a submitting state.
- Login failures show clear errors without exposing sensitive backend information.
- Phase 1 does not provide third-party login or password recovery; unavailable entries must not look usable.

### 9.3 Dashboard

The dashboard includes:

- A “Start new training” primary button.
- Recent resumes and recent JDs.
- Recent interview records.
- Training count, average score, and recent trend.
- Frequently recurring improvement areas.
- Running or failed asynchronous tasks.
- New-user empty state and first-use guidance.

When data is insufficient, use a real empty state; do not use fabricated statistics.

### 9.4 Resume list

- Show title, filename, update time, and parsing status.
- Support filtering by update time and status.
- Support uploading, renaming, deleting, and reparsing resumes.
- Show progress during parsing and the reason plus retry action on failure.
- Use vertical cards on mobile rather than forcing a desktop table.

### 9.5 Resume details

- Show title, original file, version, upload time, and parsing status.
- Display facts by categories such as personal information, education, employment, projects, and skills.
- Each fact may show a source excerpt so the user can assess parsing accuracy.
- Provide rename, reparse, and delete actions.
- Require confirmation before deletion.

### 9.6 JD list and details

- The list shows company, role, capability tags, and update time.
- Support create, view, edit, and delete.
- Create/edit fields are company, role name, and JD body.
- The JD body is suitable for pasting long text and shows a character count.
- The details page highlights capability tags extracted by the backend.

### 9.7 Question-set generation and details

When generating a question set:

- A resume is required.
- JD is optional.
- Target role is optional; recommend entering it when no JD is selected.
- Show a summary of selected materials before submission.
- Show clear progress and a notice that the user may leave while generating.

Question-set details:

- Show question order, questions, assessment intent, expected answer points, and follow-up hints.
- Support regeneration, deletion, and manual question editing.
- “Start mock interview” is the page’s primary action.

The long-term relationship between resumes and JDs is not finalized; Version 1 does not establish a complex binding model.

### 9.8 Interview history

- Separate in-progress and completed interviews.
- Show title, related question set, creation time, completion status, answer progress, and overall score.
- In-progress interviews can be continued.
- Completed interviews are view-only and do not allow answer changes.
- Support filtering by status and time.

### 9.9 Create interview

- Select one question set.
- Automatically bring in the associated resume, JD, and question count.
- Allow the interview title to be edited.
- Show timing rules.
- Clearly warn before starting that the timer cannot be paused after entering the interview.

### 9.10 Interview room

Use focus mode and show one question at a time:

- The header shows an exit entry, elapsed time for the whole interview, and overall progress.
- The main area shows the current question and necessary answer hints.
- The input area provides a multiline text input.
- The bottom provides “Previous”, “Skip”, and “Next/Submit Answer”.
- A sidebar or drawer shows question navigation and each question’s status.
- On mobile, fix the bottom action area and use a drawer for question navigation.

Question states:

- Not started
- Answering
- Answered
- Skipped

Timing rules:

- Default countdown per question is 3 minutes.
- Reference duration for the whole interview is question count multiplied by 3 minutes.
- Show the current question’s remaining time and whole-interview elapsed time together in the header.
- Give a visual warning when 30 seconds remain on the current question.
- After a question times out, continue timing or show timeout status, but do not submit or skip automatically.
- The whole interview cannot be paused after it starts.
- After refresh, restore timing from the server’s start time rather than restarting it.

Skip and re-answer rules:

- After skipping the current question, move to the next and mark the current one as awaiting an answer.
- Return to skipped questions before submitting the whole interview.
- Modify answered questions before submitting the whole interview; saving overwrites the old answer.
- When unanswered questions exist, require confirmation before submitting the whole interview.
- After completion, lock all answers; do not allow re-answering or modification.

Leave protection:

- When unsaved text exists, warn before in-page navigation or closing the page.
- Saved answers and timing state can be restored after refresh.
- When offline, retain a local draft and ask the user to confirm submission after recovery.

### 9.11 Interview report

The report includes:

- Overall score and completion status.
- Horizontal bar chart of scores by question.
- Strength summary.
- Priority improvement areas.
- Next-step training suggestions.
- For each question: the original question, user answer, score, critique, reference answer, and evidence.
- Recovery actions when report generation has failed or has not started.
- Browser printing and PDF export.

Use a single-column layout for charts and per-question content on mobile. Print styles hide navigation and non-report actions.

### 9.12 Task center

- Show task type, related object, status, progress, creation time, and completion time.
- Phase 1 mainly shows resume-parsing tasks.
- Statuses include pending, processing, succeeded, and failed.
- Failed tasks show an error summary and retry entry.
- Reserve task types for future asynchronous tasks such as ASR.

### 9.13 Settings

- Profile: display name and email.
- Appearance: light, dark, and system-following.
- Security: change password.
- Account: log out.
- Do not provide account deletion until the backend supports it.

## 10. Frontend technical solution

### 10.1 Recommended technology stack

| Capability | Choice |
| --- | --- |
| React framework | Next.js App Router |
| UI core | React + TypeScript |
| Styling | Tailwind CSS |
| Base components | shadcn/ui |
| Server state | TanStack Query |
| Forms | React Hook Form |
| Data validation | Zod |
| Theme | next-themes + CSS Variables |
| API types | goctl-generated TypeScript types/SDK |
| Unit tests | Vitest + Testing Library |
| End-to-end tests | Playwright |
| Package manager | pnpm |

Next.js remains the React application framework for the following reasons:

- The website needs good SEO and first-paint performance.
- App Router suits separation between public, authentication, and product areas.
- It provides layouts, error boundaries, loading states, and server-rendering capabilities.
- There is no need to migrate to a pure Vite SPA merely to keep React as the core.

### 10.2 State-management principles

- TanStack Query manages API data, caching, invalidation, polling, and request state.
- React Hook Form manages form state.
- Zod unifies frontend input validation and error messages.
- Prefer component state or a reducer for short-lived interaction state.
- Do not introduce Redux/Zustand by default; evaluate them only after clear cross-page client state appears.
- The server is authoritative for interview state; locally store only unsubmitted drafts.

### 10.3 API layer

- Pages and components must not scatter direct `fetch` calls.
- Wrap the generated SDK with a business API Client for:
  - Authentication injection.
  - Error-format normalization.
  - Request cancellation and timeouts.
  - Upload progress.
  - Unified 401 handling.
  - Request tracing IDs.
- Define Query Keys consistently by resource.
- Do not repeat API types manually in pages.

### 10.4 Recommended directory

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

Business components live in their corresponding feature; generic non-business components live in `components/ui`. Page files only orchestrate routes and do not contain large blocks of business logic.

## 11. General interaction standards

### 11.1 Loading states

- Use skeletons on the first page load.
- Use in-button loading for submissions so the whole page does not flash.
- Show stage information for generation/parsing operations lasting more than 2 seconds.
- Asynchronous tasks remain trackable in the task center after leaving the page.

### 11.2 Empty states

Every empty state must include:

- What is currently missing.
- Why the content is needed.
- The primary action the user can take next.

### 11.3 Error states

- Show form errors near the relevant field.
- Provide retry on page-load failure.
- Redirect authorization errors to login while preserving the return address.
- Show a global notice when offline.
- Do not expose the complete raw backend error to users.

### 11.4 Success feedback

- Use lightweight Toasts for create, save, and delete operations.
- For important workflow results, navigate to the corresponding details page.
- Do not use a Toast as a replacement for important page state.

### 11.5 Dangerous operations

Before deleting a resume, JD, or question set, show a confirmation dialog explaining the scope of impact. Use explicit copy such as “Delete this resume” rather than only “Confirm”.

## 12. Responsive design

Suggested breakpoints:

- Mobile: below 768 px.
- Tablet/small desktop: 768–1199 px.
- Desktop: 1200 px and above.

Requirements:

- Minimum supported width is 320 px.
- Mobile must not produce horizontal scrolling.
- Convert data tables into cards or summary lists on mobile.
- Convert dialogs into bottom drawers or full-screen pages on mobile.
- Touch targets are at least 44 × 44 px.
- Fixed bottom action areas must account for safe areas.
- The main actions on the interview page remain visible when the mobile keyboard is open.

## 13. Accessibility

- Every operation can be completed with a keyboard.
- Focus states are clearly visible.
- Form fields have real labels and associated errors.
- Icon buttons have accessible names.
- Color is not the only way to express state.
- Body text and key controls meet WCAG AA contrast.
- Respect `prefers-reduced-motion`.
- When a dialog opens, trap focus; when it closes, restore focus to the trigger.

## 14. Existing backend interface mapping

The following capabilities already have backend routes and can be integrated directly during the frontend refactor.

| Frontend capability | Method and path | Status |
| --- | --- | --- |
| Registration | `POST /api/v1/auth/register` | Available |
| Login | `POST /api/v1/auth/login` | Available |
| Current user | `GET /api/v1/me` | Available |
| Create resume upload | `POST /api/v1/resumes/uploads` | Available |
| Complete resume upload | `POST /api/v1/resumes/:id/versions/:versionId/complete` | Available |
| Resume list | `GET /api/v1/resumes` | Available |
| Resume details | `GET /api/v1/resumes/:id` | Available |
| Create JD | `POST /api/v1/job-descriptions` | Available |
| JD list | `GET /api/v1/job-descriptions` | Available |
| Generate question set | `POST /api/v1/question-sets` | Available |
| Create interview | `POST /api/v1/interviews` | Available |
| Interview details | `GET /api/v1/interviews/:id` | Available |
| Answer current question | `POST /api/v1/interviews/:id/answer` | Available, but insufficient for skipping and re-answering |
| Interview report | `GET /api/v1/interviews/:id/report` | Available |
| Single task status | `GET /api/v1/tasks/:id` | Available |
| Health check | `GET /api/v1/health` | Available |
| Dependency readiness check | `GET /api/v1/ready` | Available |

Beta interfaces remain in the backend but do not enter the formal navigation in this round:

- `POST /api/v1/beta/company-intel/search`
- `POST /api/v1/beta/asr/uploads`
- `POST /api/v1/beta/asr/tasks`

## 15. Backend TODO

The following interfaces are required or recommended backend capabilities for completing this specification. Final naming is subject to `.api` contract review.

### 15.1 P0: Blocks the core interview experience

#### Interview list

```http
GET /api/v1/interviews
```

Suggested support:

- `status`
- `page`
- `page_size`
- `sort`

Return interview summaries, answer progress, question count, and overall report score.

#### Save an answer for a specified question

```http
PUT /api/v1/interviews/:id/turns/:ordinal/answer
```

Rules:

- Allow first answers and overwrites while the interview is active.
- Reject edits after the interview is completed.
- Return the updated turn or the complete session.

#### Skip a specified question

```http
POST /api/v1/interviews/:id/turns/:ordinal/skip
```

Rules:

- Record `skipped_at`.
- Clear the skipped state after an answer is submitted.
- Reject the operation after the interview is completed.

#### Complete the whole interview

```http
POST /api/v1/interviews/:id/complete
```

The request may include whether the user confirms unanswered questions. Lock all answers after completion and allow report generation.

#### Server-side timing fields

Interview sessions should add:

- `started_at`
- `completed_at`
- `duration_seconds`
- `question_duration_seconds`

Interview turns should add:

- `started_at`
- `skipped_at`
- `time_spent_seconds`

`GET /interviews/:id` must return enough information for the frontend to restore timing after refresh.

### 15.2 P0: Blocks the confirmed resource-management pages

#### JD details, update, and deletion

```http
GET    /api/v1/job-descriptions/:id
PATCH  /api/v1/job-descriptions/:id
DELETE /api/v1/job-descriptions/:id
```

#### Resume update, deletion, and reparsing

```http
PATCH  /api/v1/resumes/:id
DELETE /api/v1/resumes/:id
POST   /api/v1/resumes/:id/reparse
```

Before deleting a resume, validate its related question sets and interviews and return understandable conflict information.

#### Question-set list, details, update, and deletion

```http
GET    /api/v1/question-sets
GET    /api/v1/question-sets/:id
PATCH  /api/v1/question-sets/:id
DELETE /api/v1/question-sets/:id
```

Question editing may be included in the question-set PATCH or designed as separate turns/questions interfaces.

#### Regenerate a question set

```http
POST /api/v1/question-sets/:id/regenerate
```

Clarify whether this creates a new version or overwrites the original question set. Creating a new version is recommended to preserve historical interview consistency.

### 15.3 P1: Dashboard and task center

#### Dashboard summary

```http
GET /api/v1/dashboard/summary
```

Suggested response:

- Resume count
- JD count
- Total interview count
- Recent average score
- Score trend
- Frequent improvement areas
- Recent resources
- In-progress interviews
- Active tasks

#### Task list

```http
GET /api/v1/tasks
```

Suggested support for task type, status, pagination, and time sorting.

#### Retry a failed task

```http
POST /api/v1/tasks/:id/retry
```

### 15.4 P1: User and authentication

#### Update the current user

```http
PATCH /api/v1/me
```

At minimum, support changing the display name in Phase 1.

#### Change password

```http
POST /api/v1/auth/change-password
```

#### Refresh and log out

```http
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
```

The formal product is recommended to use short-lived access tokens, refresh tokens, and HttpOnly Secure Cookies rather than storing long-lived JWTs only in `localStorage`. Before the backend session upgrade is complete, the frontend must make the temporary nature of the current authentication scheme clear.

### 15.5 P2: Future capabilities

- Password recovery and reset.
- Email verification.
- Account deletion.
- Resume version management.
- Server-side PDF export for reports.
- Productization of Beta company interview intelligence and ASR.

## 16. Frontend/backend collaboration constraints

- Modify `backend/api/interviewmaster.api` before adding any interface.
- Regenerate Go handlers, OpenAPI, and the TypeScript SDK from the contract.
- Do not maintain long-term hand-written response types that duplicate the contract.
- Standardize pagination and empty-list representation for list interfaces.
- Standardize error responses to include machine-readable `code`, user-readable `message`, and optional `details`.
- Use UTC RFC 3339 for all times; the frontend displays them in the user’s local time.
- Make deletion and interview completion idempotent or provide explicit conflict errors.

Recommended unified error structure:

```json
{
  "code": "INTERVIEW_ALREADY_COMPLETED",
  "message": "本场面试已经完成，不能再次修改回答",
  "details": {}
}
```

## 17. Testing requirements

### 17.1 Unit and component tests

At minimum, cover:

- Form validation.
- API error normalization.
- Theme switching.
- Interview timing calculations.
- Skip, edit, and completion state transitions.
- Report data display.

### 17.2 End-to-end tests

At minimum, cover:

1. Register and enter the dashboard.
2. Failed and successful login.
3. Upload a resume and see parsing results.
4. Create a JD.
5. Create a question set.
6. Create an interview, skip a question, return to answer, and complete it.
7. Refresh the interview page and restore progress and timing.
8. View and print the report.
9. Complete the core flow on mobile.
10. Re-authenticate after token expiry and return to the original page.

### 17.3 Visual regression

Establish desktop and mobile screenshot baselines for the website, dashboard, resume list, interview room, and report page, covering both light and dark themes.

## 18. Performance and quality requirements

- Do not load post-login business modules in the website’s first paint.
- Split bundles by route.
- Load large reports and long lists on demand.
- Avoid obvious layout shifts while main page content loads.
- Show real upload progress instead of an infinite spinner.
- During development, ensure lint, typecheck, unit tests, and the production build all pass.
- Lighthouse performance target is at least 85 on desktop and mobile; accessibility target is at least 90.

## 19. Staged implementation recommendation

### Phase 1: Engineering foundation and design system

- Organize App Router route groups.
- Add Tailwind, shadcn/ui, theme, and base layout.
- Establish the API Client, TanStack Query, forms, and error system.
- Complete the website, login, registration, and product shell.

### Phase 2: Productize existing capabilities

- Complete resume upload, list, and details.
- Complete JD creation and list.
- Complete question-set generation.
- Build the basic one-question-at-a-time interview and report using existing interfaces.
- Complete task-status display.

### Phase 3: Backend P0 interfaces and complete interaction

- Complete resource editing/deletion.
- Complete question-set management.
- Complete interview history.
- Complete skipping, re-answering, active completion, and server-side timing recovery.

### Phase 4: Data loop and quality

- Complete dashboard statistics.
- Complete task center.
- Complete settings and authentication upgrades.
- Add end-to-end tests, visual regression, performance, and accessibility checks.

## 20. Acceptance criteria

The frontend refactor is complete when all of the following are satisfied:

- Unauthenticated users visiting `/` see a complete Chinese product website.
- Registration, login, logout, and authentication redirects are complete.
- All confirmed pages have independent routes; the core flow is no longer centralized in one page.
- Users can complete the full “resume → JD → question set → interview → report” flow.
- The interview shows one question at a time and both a question timer and whole-interview timer.
- Before completion, users can skip, return, and overwrite answers; after completion, editing is disabled.
- Resumes, JDs, question sets, interviews, and tasks all have clear list, detail, or status pages.
- Both light and dark themes pass visual acceptance.
- There is no horizontal scrolling at 320 px, and the core flow works on mobile.
- All asynchronous operations have loading, success, failure, and empty states.
- Every missing backend capability is implemented or clearly disabled in the UI with a corresponding TODO.
- TypeScript has no unexplained `any`, and lint, typecheck, tests, and build all pass.
- Core pages pass keyboard-operation and basic accessibility checks.

## 21. Decisions pending

The following do not block Phase 1 but must be reconfirmed before the related development begins:

- Whether to establish a long-term many-to-many relationship between resumes and JDs.
- Whether question-set regeneration uses versioning or overwrite mode.
- When Beta company interview intelligence enters the formal navigation.
- Whether ASR is an answer-input method or evolves into a complete voice-interview mode.
- Whether to introduce subscriptions, usage limits, or paid capabilities.
