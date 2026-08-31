# docs document map

> Chinese index: [docs/ch/README.md](../ch/README.md)

> Compiled on 2026-08-28. Read this page before opening a specific document, so that archived checklists are not mistaken for current tasks.

The current execution checklist is [`invite-beta-todo_0828.md`](invite-beta-todo_0828.md). For the intelligent layer design, see [`agent-design.md`](agent-design.md); for local setup, see [`local-setup.md`](local-setup.md).

## Currently active

| Document | Purpose | Notes |
| --- | --- | --- |
| [`invite-beta-todo_0828.md`](invite-beta-todo_0828.md) | Remaining work for invite-only launch | **The only execution checklist**; do not add new tasks to `archive/launch-todo.md` / `archive/agent_todo.md` |
| [`agent-design.md`](agent-design.md) | Intelligent-layer principles, boundaries, and state machine | Design source; do not accumulate implementation checkboxes here |
| [`launch-redesign_0814.md`](launch-redesign_0814.md) | Product mainline: no JD/question-set management; an interview is the user unit | Product boundary remains valid |
| [`local-setup.md`](local-setup.md) | Local Compose, migrations, API/Worker/Web | Validated on Windows on 2026-08-15 |
| [`runbooks/ai.md`](runbooks/ai.md) | Model failure troubleshooting, degradation, and `IM_AI_ENABLED` | Operations entry point |

The root [`README.md`](../../README.md) keeps only one product sentence, the stack, and the shortest startup commands; use this directory for details.

## Archived (`archive/`)

Expired task lists, old productization TODOs, and OceanBase / multi-service drafts are archived here. See the full explanation in [`archive/README.md`](archive/README.md).

Archived in this round:

- [`archive/technical-plan_v1.md`](archive/technical-plan_v1.md): historical engineering baseline; it must not be used as the current product plan.
- [`archive/agent-architecture-api-refactor-todo_0814.md`](archive/agent-architecture-api-refactor-todo_0814.md): V2 implementation record; it must not be used as the current task checklist.

Do not schedule or accept work according to these files. Open them only when you need to compare “how we thought about it at the time.”

## Organization conclusion (delete / merge / keep)

**Nothing was deleted directly.** These files still have historical comparison value; deleting them would break search results and PR discussions. All outdated content goes into `archive/`, with the file header marking it “not a current task.”

**Long documents were not merged into one.** `full-development-outline.md`, `phase0-phase1-schedule.md`, and `requirements-outline.md` overlap, but they are all outdated choices (OceanBase, LangChainGo, seven microservices, JD/RAG). Combining them would only make them harder to read. The same applies to `frontend-refactor-*` and `backend-refactor-todo`: those describe the 07-29 JD + question-set productization and have been superseded by 0814.

**The following can be treated as “completed; do not execute again”:**

| File | Reason |
| --- | --- |
| `archive/launch-todo.md` | 2026-08-07; it still says “all AI is stubbed,” which is contrary to the current state and **the most dangerous** |
| `archive/acceptance-checklist.md` | M0–M3/Beta; includes JD creation, five fixed questions, and SHA-256 vectors |
| `archive/frontend-refactor-*.md`, `backend-refactor-todo.md` | 07-29 productization; the main flow is outdated; the current contract is `backend/api/interviewmaster.api` |
| `archive/agent_todo.md` | Most Agent infrastructure items are `[x]`; unfinished items (invite gate, Golden/E2E gates) have moved into the invite-only TODO |
| `archive/full-development-outline.md`, `phase0-phase1-schedule.md`, `requirements-outline.md` | Explicit historical drafts; the documents themselves say “do not use as the current baseline” |
| `archive/PROJECT_WALKTHROUGH.md` | Walkthrough from 08-06; it still explains the JD/question-set flow |

**Guidance for future documentation changes:**

1. Put new tasks only in `invite-beta-todo_0828.md`, or open `docs/yyyyMMdd_topic.md` and add a row under “Currently active” on this page.
2. Put design changes in `agent-design.md`; do not copy them into the TODO.
3. When you check an item off, update the corresponding file; do not let a second checklist and the code tell different stories.
