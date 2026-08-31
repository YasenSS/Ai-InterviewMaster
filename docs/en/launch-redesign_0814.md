# InterviewMaster Launch Redesign (08-14)

> Created on 2026-08-14. Records the product mainline, module decisions, and migration plan for this round. The old [`PROJECT_WALKTHROUGH.md`](archive/PROJECT_WALKTHROUGH.md) has been archived. See [`invite-beta-todo_0828.md`](invite-beta-todo_0828.md) for the remaining invite-only work.

## Contents

1. Product positioning and core flow
2. Target architecture and user journey
3. Modules to retain, merge, hide, or delete
4. Interview orchestration and state model
5. Frontend and backend refactoring plan
6. Migration sequence and acceptance criteria

## 1. Product Positioning and Core Flow

InterviewMaster is not primarily for users to manage technical resources such as resumes, JDs, question sets, and tasks. It is for job seekers to start a targeted AI mock interview immediately with as little configuration as possible.

The confirmed first-version main flow is:

1. The user uploads a resume, and the system parses it and extracts facts in the background.
2. The user selects a primary technical language, such as Java, Go, or C++.
3. The first version fixes the position as backend development; frontend, client, and Agent development roles may be added later.
4. The user selects a target company.
5. The backend generates a question outline and an internal question set based on the resume, technical language, position, and target company.
6. The question set is not exposed as a frontend-manageable resource; after clicking “Start Interview,” the user enters the interview directly.
7. The interview model node combines the question outline, current progress, and the user’s answer to choose a follow-up, an extension question, or the next examination point.

Core product principles:

- The user creates and experiences “one interview,” not “one question set.”
- The question outline, question collection, and follow-up branches are internal implementation details of interview orchestration.
- The configuration page collects only the information required to generate a high-quality interview.
- Starting should feel as close to immediate as possible; the background-generation process is represented through a preparing state, without exposing task-center or question-set-management concepts.

## Confirmed Module Adjustment Records

### R-001 Delete the JD/Job-Description Module

- Delete all frontend JD/job-description entries, navigation items, list, detail, create, and edit pages.
- Delete backend JD/job-description interfaces.
- During subsequent refactoring, continue checking and cleaning up request fields, service logic, database relationships, and generation-workflow inputs bound to JDs.
- Fix the first-version position as “backend development”; users no longer need to create or select a JD.

### R-002 Add Interview Records

- Add an “Interview Records” entry and an independent page under the frontend “Mock Interview” module.
- Each interview record must save and display the complete process of one interview, rather than only a summary or final score.
- The complete process must include at least:
  - All questions asked by the model, including extensions and follow-ups generated from the answers;
  - The user’s actual answer to each question;
  - The score corresponding to each round;
  - Analysis or evaluation of the user’s answer;
  - The corresponding reference/improved answer.
- Interview records should support reviewing one complete interview at a time. The detailed page structure and its relationship with the existing “review report” remain to be discussed later.

### R-003 Turn Question Sets into Internal Backend Artifacts

- Remove the user’s awareness of and independent management flow for question sets.
- The frontend no longer provides question-set navigation, list, detail, edit, delete, or regeneration pages.
- After the user clicks “Start Interview,” the backend generates the question outline and internal question collection in the background and automatically enters the interview.
- The model node decides whether to extend, follow up, or switch to the next examination point based on the question outline, current interview progress, and the user’s answer.

## 2. Target Architecture and User Journey

```text
Upload and parse resume
  → Select primary technical language
  → Select target company
  → Click Start Interview (position fixed as backend development)
  → Interview enters preparing; backend generates blueprint and candidate questions
  → Automatically enter active when the first question is ready
  → Decide after each answer: follow up / switch capability / finish
  → Score asynchronously after completed
  → Review the complete Q&A and review in interview records
```

The frontend does not display question-set IDs, asynchronous task IDs, JDs, or generated-resource management flows. Preparing and report-generation states are both represented on the current business page.

## 3. Modules to Retain, Merge, Hide, or Delete

| Original module | Adjustment | Responsibility after refactoring |
| --- | --- | --- |
| Resume | Retain | Upload, parse, and view interview evidence material |
| JD/job description | Delete | First-version position fixed as backend development |
| Question set | Convert to internal implementation | Store the interview blueprint and candidate questions without public CRUD |
| Mock interview | Rework entry point | Select resume, language, and company, then start directly |
| Interview records | Add | List and complete per-interview Q&A, scoring, comments, and reference answers |
| Task center | Remove from the product UI | Single-task status is used only for polling and retrying within business pages |
| Dashboard | Simplify | Show real training progress, score trends, and areas for improvement |

## 4. Interview Orchestration and State Model

Interview states:

```text
preparing ──generation succeeds──> active ──model/user ends──> completed
    └──────generation fails──────> failed
```

Dynamic rounds save only questions that have actually been asked. Backend candidate questions are not copied to the frontend in advance and are not counted as public rounds in advance.

After each answer, the interviewer decision-maker may choose only:

- `follow_up`: Continue probing the current capability point and generate the next concrete question;
- `next_capability`: Select the next examination point from the blueprint and have the system read the corresponding candidate question;
- `finish`: End the interview when the minimum number of rounds has been reached and the information is sufficient.

Follow-ups are constrained by the budget for the entire interview and the maximum depth of a single question chain. If the model output is invalid, times out, or goes out of bounds, the system safely falls back to the next examination point; the interview must not become stuck or loop indefinitely.

## 5. Frontend and Backend Refactoring Plan

Backend:

- The create-interview interface directly accepts `resume_id`, `primary_language`, and `target_company`;
- Creating an interview also creates the internal blueprint carrier and generation task;
- Delete JD interfaces, database relationships, and storage tables;
- Delete public question-set interfaces while retaining internal candidate questions and evidence relationships;
- Real interview rounds are associated with their internal source questions; dynamic follow-ups may have no source question;
- Reports score real rounds and must not associate them with candidate questions through mutable question numbers.

Frontend:

- “Start Interview” is the only training entry point;
- The `preparing` page polls interview status automatically and enters the first question after success;
- The interview room displays only the question that has already been asked in the current round;
- “Interview Records” provides a history list and complete per-round review;
- Delete JD, question-set, and task-center navigation and pages.

## 6. Migration and Implementation Results

This round of refactoring has been completed:

- The API, OpenAPI, and generated frontend client have removed JD, public question-set, and task-list contracts; single-task queries and retries are used only internally by resume, interview preparation, report, and other business pages.
- Creating an interview creates the internal candidate-question carrier, a `preparing` interview, and a preparation task in the same database transaction; the backend activates the interview only after the first question is ready.
- Each interview fixes a `resume_version_id`; question generation, dynamic decisions, and scoring always use the same resume fact snapshot, ensuring that historical records can be reproduced.
- The dynamic interviewer permits only the three bounded actions `follow_up`, `next_capability`, and `finish`; the maximum single-chain follow-up depth, total interview follow-up budget, and minimum rounds before finishing are all constrained by the server.
- Answers use a row lock on the current round and a no-overwrite rule; if an answer has been saved but progression is interrupted, reading interview details triggers deterministic recovery to prevent the session from becoming permanently stuck.
- Preparation and report Workers add task claiming, tenant and resource-association validation, successful idempotency, timeout-task takeover, and failure-state compensation; business pages support in-place retries.
- The interview-record list supports server-side pagination, and complete records display each round’s question, answer, score, comments, reference answer, and evidence.

Migration `00009_launch_redesign.sql` permanently deletes the JD table. A full `up → down → up` cycle has been completed in an isolated PostgreSQL + pgvector environment. A v8 sample containing JDs, candidate questions, and pre-created rounds was used to verify that the company and resume version were filled correctly, displayed rounds retained their scoring sources, and undisplayed future rounds were deleted. The production execution still requires a database backup; rollback can restore only empty compatibility tables, not deleted JD content.

## 7. Acceptance Results

- [x] The database migration upgrades an existing v8 database to the refactored version.
- [x] The API contains no JD, public question-set, or task-list routes.
- [x] Users can create an interview directly with a resume, primary language, and target company.
- [x] A successful preparation automatically enters the interview; a failed preparation can be viewed and retried in place on the business page.
- [x] The model can follow up, switch examination points, or finish within bounds; failures have deterministic fallback behavior.
- [x] The frontend has no JD, question-set, or task-center product entries.
- [x] Interview records display questions, answers, scores, comments, and reference answers round by round.
- [x] The interview-record list supports pagination and does not display only the first 20 interviews.
- [x] Frontend and backend static checks, tests, and production builds all pass.
