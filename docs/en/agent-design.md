# InterviewMaster Agent Design

> Initial date: 2026-08-12
> Revision date: 2026-08-14
> Positioning: target design and implementation constraints for the intelligent layer (Agent / LLM / orchestration)
> Product boundary: the user uploads a resume, selects a primary technical language and target company; the initial release fixes the role as backend development
> See [`invite-beta-todo_0828.md`](invite-beta-todo_0828.md) for the remaining invite-only implementation items. Do not accumulate implementation checkboxes in this file.

---

## 0. One-sentence conclusion

**Do not build autonomous multi-Agent systems; use “deterministic business workflows + one narrowed real-time interviewer ReAct loop.”**

- Resume extraction, blueprint generation, final scoring, and report generation are known steps orchestrated by deterministic workflows.
- The only part of the entire system that is close to an autonomous Agent is the interviewer node that runs after each user answer.
- The interviewer can look up read-only materials, analyze the answer, choose whether to deepen or switch capabilities, and generate the next concrete question.
- The server-side state machine decides when the interview ends; the model can only recommend finishing.
- The internal question set provides the blueprint, capability Rubric, assessment materials, and failure fallback. It is not a fixed list played in order and cannot become the upper bound on interview turns.
- The MVP does not need multiple Agents, cross-session long-term memory, or generic context compression.

---

## 1. Product goals and boundaries

### 1.1 Main user flow

~~~text
Upload and parse resume
  → Select primary technical language
  → Select target company
  → Click start interview (role fixed as backend development)
  → Generate interview blueprint, capability Rubric, fallback materials, and first question in the background
  → User answers
  → Interviewer Agent generates the next question in real time based on the answer
  → State machine decides whether to continue or finish based on turns, time, and capability coverage
  → Generate per-turn scoring and a complete review asynchronously
~~~

The user creates and experiences “one interview,” not a JD, question set, or generic asynchronous task.

### 1.2 Fixed constraints for the initial release

- The role is fixed as backend development and no longer depends on a JD module.
- Interview inputs are: resume version, primary technical language, target company, and the fixed role profile.
- Use standard interview mode by default; users should not need to configure complex parameters.
- Default standard-mode policy:
  - Minimum 8 turns;
  - Target approximately 12 turns;
  - Maximum 16 turns;
  - Time budget of 30 minutes;
  - At most 2 consecutive follow-ups on one main question chain;
  - Default total follow-up budget of 4.
- These numbers are server-side safety boundaries and default policies. Fast, standard, and deep modes may be added in the future, but the numbers must not be scattered as hard-coded values across Prompts, Schemas, and business logic again.

### 1.3 AI runtime mode must be genuinely visible

- `AI.Enabled=true`: real AI interview mode; every normal-path turn must produce an auditable model call.
- `AI.Enabled=false`: rule-demo mode, used only for local development or failure drills.
- The frontend must clearly identify rule mode; users must not believe they are receiving an AI interview.
- If the model is not configured in production, startup should fail or creation of AI interviews should be disabled; the system must not silently pretend to be in normal mode.

---

## 2. Overall architecture: specialized nodes, not multiple Agents

### 2.1 Node breakdown

| Node | Input | Output | Orchestration |
| --- | --- | --- | --- |
| Resume fact extraction | Tika text | Skills, projects, metrics, and source-text evidence | Chain |
| Interview blueprint generation | Resume facts + language + company + fixed role profile | Capability plan, turn range, difficulty curve, and time budget | Chain |
| Internal-material generation | Blueprint + resume facts + company materials | Capability Rubric, expected evidence, anchor questions, and fallback questions | Graph |
| First-question generation | Blueprint + current capability | The first real interview question and its scoring snapshot | Chain |
| **Real-time interviewer** | Blueprint progress + recent Q&A + current answer | Deepen, switch capability, or recommend finishing; must include a next question when not finishing | **Narrowed ReAct** |
| Final per-turn evaluation | Complete real Q&A + question scoring snapshots + resume evidence | Five-dimensional score, comments, and reference answer | Workflow |
| Report writing | All per-turn evaluations + capability coverage | Overall review, strengths, improvement areas, and training plan | Chain |

### 2.2 Framework boundaries

- Continue using Eino, but Eino may exist only inside `backend/internal/platform/ai/`.
- The business layer depends only on in-house interfaces such as `ChatModel`, `Tool`, and `InterviewerAgent`.
- Domain logic must not import Eino directly, so that the Provider or orchestration implementation can be replaced later.
- Use the Eino Graph for branched generation, validation, deduplication, and coverage checks.
- Use the Eino Workflow for multi-stage scoring with clear field mappings.
- Use Eino ADK or an equivalent internal implementation only for the real-time interviewer ReAct.
- The framework choice is not an acceptance target; “call the real model on every turn and generate the next question” is the product acceptance target.

---

## 3. Interview blueprint and internal question set

### 3.1 The blueprint is an executable policy

The blueprint cannot be only archived JSON; it must directly drive the state machine. Suggested structure:

~~~json
{
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
      "weight": 25,
      "target_evidence": 2,
      "difficulty_curve": ["medium", "hard"],
      "rubric": ["individual contribution", "technical tradeoffs", "quantified results"]
    }
  ]
}
~~~

The blueprint must control:

- Minimum, target, and maximum total turns;
- Priority of high-weight capabilities;
- The number of valid evidence items required for each capability;
- The current question difficulty and the next-step difficulty;
- Per-chain follow-up depth and the total-session follow-up budget;
- Time budget and final completion conditions.

Do not retain the rule that “every interview has exactly 5 questions.” The turn range can only be read from the blueprint policy.

### 3.2 The correct responsibilities of the internal question set

The internal `question_set` may continue to exist, but it stores only:

- Interview blueprint;
- Capability Rubric;
- Expected answer evidence for each capability;
- A small number of anchor questions;
- Fallback questions for when the model is unavailable;
- Prompt, model, and input snapshot information.

Internal questions must not:

- Act as a queue played in order during a normal interview;
- Decide how many turns an interview can have;
- Be copied in advance into every public interview turn;
- Be exposed by the frontend as user-manageable resources.

On the normal path, every question actually asked must be generated by the real-time interviewer and written to `interview_turns`. Only the degradation path may use internal fallback questions.

---

## 4. Real-time interviewer main loop

### 4.1 State model

~~~text
preparing
  → active(answering)
  → active(deciding)
  → active(answering)
  → ...
  → completed

preparing/deciding failure
  → retryable
  → explicit degradation or failure
~~~

`active` is the public business status; `answering` / `deciding` can be internal phases. When an answer has been persisted but the next question has not yet been generated, the session must be in a recoverable `deciding` state. Recovery must not depend on guessing from a timeout.

### 4.2 Main loop

~~~text
prepare:
    lock resume_version_id
    blueprint = generate executable blueprint
    materials = generate Rubric, anchors, and fallback materials
    first_question = interviewer generates the first question
    save the complete snapshot of the first question

while session.status == active:
    save the user's answer
    session.phase = deciding

    context = {
        blueprint,
        capability_progress,
        elapsed_time,
        recent_turns,
        current_question,
        current_answer,
        pinned_resume_facts
    }

    decision = interviewer_agent.react(context, readonly_tools, max_tool_calls=2)

    if policy.should_finish(decision, context):
        complete the interview
    else:
        validate the next question in decision
        update capability progress
        save the complete snapshot of the next question
        session.phase = answering
~~~

### 4.3 Agent decision contract

Converge the decision actions to:

~~~text
deepen             Continue probing around the current answer
switch_capability  Switch to a capability with insufficient coverage
recommend_finish   The model believes that further questioning has low value
~~~

Both `deepen` and `switch_capability` must return one concrete next question and include:

- `question`
- `turn_kind`
- `capability_key`
- `intent`
- `expected_points`
- `difficulty`
- `evidence_fact_ids`
- `reason`
- `coverage_observation`

`recommend_finish` is only a recommendation; the model cannot directly modify Session state.

### 4.4 The state machine owns the right to finish

The server-side completion policy checks the following in order:

1. Reaching `max_turns`: force completion.
2. Reaching the time budget: finish after the current turn.
3. Fewer than `min_turns`: finishing is forbidden.
4. A high-weight capability remains uncovered: finishing is forbidden.
5. Reaching `target_turns` with overall evidence coverage meeting the threshold: finishing is allowed.
6. If the model recommends finishing and coverage conditions are met, finish earlier than `target_turns`, but never below `min_turns`.

Invalid model output must not change these boundaries.

### 4.5 One logical run produces one final decision

Each turn allows at most two read-only tool calls, but the model output after the tool loop ends should directly become the structured decision.

Do not use:

~~~text
First call the model to decide whether to use a tool
  → regardless of whether a tool is called
  → call the model again to generate the same decision
~~~

Without tool calls, one interviewer turn should normally have only one model request; with tool calls, additional requests may come only from a real tool-result feedback loop.

---

## 5. Tool calls

### 5.1 MVP read-only tools

| Tool | Purpose | Data boundary |
| --- | --- | --- |
| `lookup_resume_facts` | Verify the candidate's projects, skills, and metrics | Must be bound to the current Session's `resume_version_id` |
| `lookup_company_intel` | Query locally maintained target-company and role-assessment materials | Read-only public materials, with a result-count limit |

Public interview-experience search can be added later, but the model must not be allowed to provide `user_id`, tenant ID, or resume version ID on its own.

### 5.2 ReAct boundaries

- At most 2 tool calls per turn.
- Tools are read-only and do not require permission confirmation.
- Tool results must be trimmed before being fed back.
- A tool failure should not directly end the session; without the tool result, the model may continue by generating a question that does not invent facts.
- Tool calls, the final decision, and the corresponding Turn must be linkable through invocation records.

---

## 6. Context, state, and capability progress

### 6.1 Use state; do not build generic long-term memory

PostgreSQL is the reliable state source for the interview Agent:

- `interview_sessions` stores the blueprint, policy, current phase, and progress;
- `interview_turns` stores each question actually asked and the user's answer;
- Capability progress stores the evidence already obtained and coverage for each capability;
- Reassemble context from the database before every model call.

There is no need for Claude Code-style generic long-term memory, nor to put the complete history permanently into the model context.

### 6.2 Per-turn context

The model input includes only:

- System rules;
- Fixed role profile, primary language, and target company;
- A summary of the current blueprint and capability progress;
- The current question and user answer;
- The most recent 2–3 Q&A turns;
- Turns used, follow-ups, and remaining time;
- Resume fact IDs pinned by the current Session;
- Evidence obtained through a tool query when necessary.

Trim recent Q&A; do not use an LLM to summarize the context. Capability progress is structured state and does not rely on model memory.

### 6.3 Capability progress

Record at least the following for each capability:

~~~text
asked_turns
follow_up_turns
evidence_count
evidence_quality
coverage_score
last_difficulty
unresolved_gaps
~~~

The model may provide `coverage_observation`, but the server must validate it and limit the update range. The model must not arbitrarily modify turn counts, budgets, or the state of other capabilities.

---

## 7. Evaluation and reports

### 7.1 Lightweight online observation

To decide the next question, the interviewer must produce a lightweight observation on every turn:

- Whether the answer is on topic;
- Whether it provides a concrete personal action;
- Whether it contains verifiable evidence;
- What information is still missing;
- Whether the current capability is worth probing further.

Put this together with the next question in the same final Agent output, avoiding an additional independent scoring call on every turn.

### 7.2 Formal scoring after the interview

Formal scoring still runs asynchronously after the interview ends:

~~~text
Intent
  → Evidence
  → Five-dimensional Evaluation
  → Critique / Golden Answer
~~~

Each turn's scoring must read the question snapshot stored by `interview_turns` itself, rather than relying on a candidate-question number or a mutable internal question set.

Scoring dimensions, total-score calculation, and report writing belong to a deterministic Workflow, not to the interviewer Agent main loop.

---

## 8. Data-model requirements

### 8.1 Interview Session

In addition to existing fields, the Session must have or be able to read stably from the blueprint:

~~~text
resume_version_id
agent_mode
phase
min_turns
target_turns
max_turns
time_budget_minutes
max_follow_up_depth
max_follow_ups_total
follow_ups_used
current_capability_key
capability_progress
interviewer_model
~~~

The same Session must pin the resume version, Prompt version, and model version.

### 8.2 Interview Turn

Every real Turn must store an immutable snapshot:

~~~text
question
answer
turn_kind
parent_turn_id
capability_key
intent
expected_points
difficulty
evidence_fact_ids
decision_reason
coverage_observation
model_invocation_id
source_question_id (optional only for fallback or anchor questions)
~~~

Dynamically generated main questions and follow-ups may have an empty `source_question_id`, but they cannot omit the snapshot fields required for scoring.

### 8.3 Decision idempotency and recovery

- After saving an answer, there must be a persisted `deciding` state or a unique decision record.
- The same answered Turn may successfully generate a next question only once.
- After a Worker/API crash, the same decision can be recovered from state; the answer must not be overwritten and a duplicate question must not be inserted.
- Writing the next question and updating the Session phase should happen in the same transaction.

---

## 9. Prompt governance

~~~text
backend/prompts/
  resume.extract/v1/
  blueprint.generate/v2/
  interview.materials/v1/
  interview.next_turn/v2/
  evaluation.critique/v2/
  report.compose/v1/
~~~

- System, task, evidence, and user input must be clearly separated.
- Resumes, company materials, and user answers must always be injected as untrusted data.
- Every output uses JSON Schema and Go domain validation.
- Structured-output failure may be repaired at most once.
- Audit Prompts by `prompt_key + version`; do not overwrite the meaning of an existing version directory.
- The next-question Prompt must require both `deepen` and `switch_capability` to output a concrete question.
- Prompts must not hard-code the global question count; they read only the blueprint boundaries passed by the server.

---

## 10. Audit, cost, and model pinning

### 10.1 Call audit

`model_invocations` must record at least:

- provider / model;
- prompt key / version;
- session / task / resource;
- input and output hashes;
- tokens and estimated cost;
- latency and error code;
- trace ID / request ID;
- corresponding Turn or decision ID.

Do not write model bodies directly to the audit table, to avoid exposing resumes and answers.

### 10.2 Cost constraints

- Each Agent turn uses “one logical run” to avoid duplicate model calls when no tools are used.
- At most 2 tool calls.
- Include only the most recent 2–3 turns and structured progress in the input.
- Final per-turn scoring may run in controlled parallel, but concurrency must be limited.
- Daily call quotas must be recalculated from the complete cost of a normal interview; a normal interview must not hit a hard limit halfway through.
- At a soft quota, the Session may switch to an already allowed smaller-model policy, but the same Session should not switch models frequently without notice.

### 10.3 Pin the model within a Session

Determine these when creating the interview:

~~~text
interviewer_model
prompt_version
policy_version
~~~

Read and validate them from the Session on every subsequent turn; do not record only the model name from the last response.

---

## 11. Frontend real-time experience

The core definition of “real-time questioning” is: **the next question is generated based on the current answer after the user submits it.** It does not mean that the output must be streamed character by character.

Recommended interaction:

~~~text
User submits an answer
  → Answer is saved immediately
  → Page displays “The AI interviewer is analyzing your answer”
  → Session phase = deciding
  → Next question finishes generating
  → Page receives turn.ready and displays the new question
~~~

Implementation can proceed in two steps:

1. First use an asynchronous decision task + short polling for the MVP, ensuring crash recovery and avoiding request timeouts.
2. Then upgrade to SSE to push `turn.ready`; introduce streaming ASR, TTS, and WebSocket only when building voice interviews in the future.

The frontend must also show:

- Whether the current mode is real AI or a rule demo;
- Whether the model is thinking, retrying, or degraded;
- A degraded question must not pretend to have been generated by AI from the current answer.

---

## 12. RAG and extension capabilities

### 12.1 The MVP does not need generic RAG

The size of one full resume and its structured facts is small enough to place directly into the preparation Prompt. The real-time interviewer only needs a read-only tool to query facts for the current resume version.

### 12.2 Future extension point

Only after the public interview corpus reaches a sufficiently large scale should the system introduce:

- Embedding;
- pgvector Retriever;
- Company, role, and topic filters;
- Strict isolation between private resume facts and public interview experiences.

RAG, MCP, and more tools are not prerequisites for fixing the current main loop.

---

## 13. Recommended directory locations

~~~text
backend/internal/platform/ai/
  model.go
  interviewer_agent.go
  react.go
  structured.go
  audit.go
  provider/
  tools/

backend/internal/aiworkflow/
  resume.go
  blueprint.go
  materials.go
  next_turn.go
  evaluation.go
  report.go

backend/apps/api/internal/logic/workspace/
  createinterviewlogic.go
  interviewer.go

backend/apps/worker/internal/tasks/
  questiongen.go
  nextturn.go
  reportgen.go
~~~

- `platform/ai` handles how models are called and orchestrated.
- `aiworkflow` handles strongly typed intelligent nodes.
- The API handles business transactions and state transitions.
- The Worker handles recoverable asynchronous preparation, next-turn generation, and reports.

---

## 14. Refactoring order

~~~text
1. Change the Blueprint and NextTurnDecision contracts; remove the fixed five-question rule
2. Add Session policy/phase/capability_progress and Turn snapshot fields
3. Change the internal question set into Rubric, anchors, and fallback materials
4. Rewrite the real-time interviewer: every non-finish action generates a concrete next question
5. Change post-answer progression to a recoverable deciding state machine
6. Correct the tool scope and bind it to the current resume_version_id
7. Change reports to read Turn snapshots directly
8. Add thinking, retry, AI/rule mode, and turn.ready experiences to the frontend
9. Connect a real model and complete end-to-end acceptance
10. Then consider refining Eino Graph/Workflow and adding SSE/voice extensions
~~~

Do not use the following “surface fixes”:

- Only change 5 to 10 or 15;
- Only enable `IM_AI_ENABLED`;
- Only add WebSocket or streaming animation;
- Continue letting `next_capability` choose the next question from a fixed candidate list.

These approaches do not turn a question-list state machine into a real-time interview Agent.

---

## 15. Acceptance criteria

### 15.1 Main loop

- [ ] In AI mode, every answered Turn has a corresponding model-call audit record.
- [ ] The next question can reference or respond to the current answer rather than following a fixed question-list order.
- [ ] Both `deepen` and `switch_capability` generate a concrete next question.
- [ ] A standard interview can naturally exceed 5 turns.
- [ ] The interview never exceeds `max_turns` or the time budget.
- [ ] The interview cannot finish before `min_turns` or before key capabilities are covered.
- [ ] Per-chain follow-up depth and the total-session follow-up budget take effect.

### 15.2 State and recovery

- [ ] After an answer is saved, the session enters a persisted `deciding` state.
- [ ] Retrying does not overwrite the answer or generate a duplicate next question.
- [ ] An API/Worker restart can recover an unfinished decision.
- [ ] The same Session pins the resume, model, Prompt, and policy versions.

### 15.3 Scoring and records

- [ ] Dynamic main questions and dynamic follow-ups both save complete scoring snapshots.
- [ ] The report does not depend on candidate-question ordinal associations.
- [ ] The interview record fully displays questions, answers, scores, comments, and reference answers.

### 15.4 Runtime authenticity

- [ ] When AI is disabled, the frontend clearly displays rule-demo mode.
- [ ] During real AI acceptance, `model_invocations` is non-zero.
- [ ] At least one real Provider completes the “preparation → multi-turn dynamic Q&A → report” flow.
- [ ] Model failure, timeout, invalid output, and tool failure all have clear observable results.

---

## 16. Core design decisions at a glance

| Question | Decision |
| --- | --- |
| Multiple Agents? | No; deterministic workflows + a single narrowed interviewer ReAct |
| Who generates the normal next question? | The real-time interviewer Agent |
| What does the internal question set do? | Blueprint, Rubric, anchors, expected evidence, and fallback materials |
| Fixed five questions? | No; controlled by the blueprint turn range and state machine |
| Who decides when to finish? | The server-side state machine; the model can only recommend |
| Default interview length? | Standard 30 minutes; minimum 8, target 12, maximum 16 turns |
| Tool calls? | Read-only, at most 2 per turn, bound to the Session data boundary |
| Context? | Blueprint progress + current answer + the most recent 2–3 turns |
| Long-term memory? | Not in the MVP; use DB state and an optional skill profile |
| RAG? | Not needed for the MVP; connect it after the public interview corpus grows |
| Does real-time mean character-by-character streaming? | No; the core is dynamic generation of the next question after answer submission |
| What happens when AI is off? | Clearly display rule-demo mode; do not present it as AI |
