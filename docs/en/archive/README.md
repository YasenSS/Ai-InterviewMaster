# Archived Documents

> These files are **not** the basis for current development or the invite-only launch. They are retained to provide context on “why it was designed this way at the time.”
> Current tasks: [`../invite-beta-todo_0828.md`](../invite-beta-todo_0828.md). Documentation map: [`../README.md`](../README.md).

Relative paths: when files in this directory mention `agent-design.md`, look for it in the parent `docs/en/` directory; `technical-plan_v1.md` is in the same `archive/` directory as this page.

## Why these files are here

| File | Original role | Reason for archiving |
| --- | --- | --- |
| `launch-todo.md` | 08-07 production-launch gap list | It still says “all AI is stubbed,” which contradicts the code; its security items are also mixed with public-registration standards |
| `agent_todo.md` | 08-12 Agent infrastructure TODO | Most P0 items were checked off; the remaining Golden/E2E/gating work was moved into the invite-only TODO. It has not been used as the main-loop acceptance basis since 0814 |
| `acceptance-checklist.md` | M0–M3/Beta local acceptance | It contains JD, five fixed questions, fake vectors and a generic Task API |
| `PROJECT_WALKTHROUGH.md` | 08-06 project walkthrough | Its architecture narrative still follows JD/question-set flows; since 0814 it has been treated as a historical record |
| `frontend-refactor-requirements.md` | 07-29 frontend product requirements | The main flow includes JD and question-set management |
| `frontend-refactor-todo.md` | Frontend tasks from the same effort | It still says “to be developed,” while the pages and routes are outdated |
| `backend-refactor-todo.md` | 07-29 backend contract draft | The current contract is `backend/api/interviewmaster.api` |
| `full-development-outline.md` | Early technical draft | OceanBase, LangChainGo and seven microservices; the document itself says not to use it as a baseline |
| `phase0-phase1-schedule.md` | Early schedule | Same as above; the tasks no longer correspond to code |
| `requirements-outline.md` | Early requirements (RAG/intelligence) | The product boundary follows 0814 |

## Why they were not merged

The three “early outlines” and the three “07-29 refactor” documents refer to one another internally and duplicate content, but merging them into one extremely long historical document would not improve readability. They are archived under their original filenames.
