# InterviewMaster Launch TODO

> **Archived (2026-08-28). The content is outdated (it still says AI is stubbed); do not use it as a task list.** Current checklist: [`../invite-beta-todo_0828.md`](../invite-beta-todo_0828.md).
> Generated: 2026-08-07
> Basis: inventory of the code at that time against [`technical-plan_v1.md`](technical-plan_v1.md)
> Goal: list the gaps between an “engineering closed-loop demo” and a product ready for real launch

---

## Overview

| Dimension | Current state | Gap severity |
| --- | --- | --- |
| Engineering foundation (authentication/state machine/async/isolation/contracts) | ✅ Basically complete | Low |
| AI intelligence layer (model calls/Prompts/retrieval/evaluation/cost) | ❌ All stubbed | **Extremely high** |
| Security and compliance | ⚠️ Partial (bcrypt, no CSRF, no virus scanning, no data export/deletion) | High |
| Rate limiting and quota governance | ❌ None | High |
| Observability | ⚠️ OTel skeleton exists but is disabled by default, with no alerts | Medium |
| Deployment and operations | ⚠️ CI only has lint/test/build; no deploy pipeline | Medium |
| Test depth | ⚠️ Mainly unit tests; integration/E2E/security tests are missing | Medium |

---

## P0 — Blocking: cannot launch without these

### AI core (largest gap)

- [ ] **Define model capability interfaces**: `ChatModel` (Generate/Stream), `EmbeddingModel`, `Reranker`, `SpeechToTextModel`; the business layer must not know provider SDKs
- [ ] **Integrate at least one real LLM Provider** (OpenAI-compatible / domestic platform / local model), including timeout, retry and fallback
- [ ] **Version Prompts**: `prompt_key + prompt_version`; store templates under `prompts/` or in the DB, with rollback support
- [ ] **Connect question generation to a real model**: replace the fixed five questions in `createquestionsetlogic.go` with questions generated from resume facts + JD, including evidence citations
- [ ] **Connect scoring to a real model**: replace character-count scoring in `getinterviewreportlogic.go` with the multi-stage Intent → Evidence → Evaluation → Critique design
- [ ] **Connect reporting to a real model**: have the model generate per-question feedback, Golden Answers and a training plan, with references locatable in the original resume
- [ ] **Real Embedding**: replace the SHA-256 fake vectors in `resumeparse.go` with a real embedding service and pin the Embedding Space version
- [ ] **Real retrieval**: pgvector retrieval filtered by `workspace_id` plus hybrid full-text/keyword retrieval
- [ ] **Structured-output validation**: put model output into Go Structs and validate against JSON Schema; allow only one repair retry, then return an explicit error

### Security baseline

- [ ] **Replace password hashing with Argon2id** (currently bcrypt)
- [ ] **CSRF/Origin validation** (currently only SameSite Cookie, with no explicit CSRF token/Origin validation)
- [ ] **Basic API rate limiting**: protect login against brute force and rate-limit business APIs per user
- [ ] **File virus scanning** (currently only magic bytes + size are checked; the design requires virus scanning)
- [ ] **User data export/deletion entry points**: delete resumes, sessions, audio and accounts, and record deletion-task status (listed as a mandatory MVP deliverable in the design)

---

## P1 — Experience and stability (add soon after launch)

### AI governance

- [ ] **Model-call audit**: `model_invocations` table recording provider/model/prompt_key/version/input-output hashes/tokens/cost/latency/trace_id
- [ ] **Cost controls**: maximum input/output per call, timeout, per-user concurrency limits; cache resource versions to avoid duplicate generation
- [ ] **Quota system**: workspace daily/monthly budget soft/hard thresholds; degrade above the soft threshold and reject above the hard threshold
- [ ] **Prompt Injection defense**: separate system rules/tasks/evidence/user input; include malicious-prompt cases in the evaluation set

### Observability

- [ ] **Actually connect OTel**: deploy Collector + Jaeger/Prometheus and enable it in production
- [ ] **AI-specific metrics**: time to first token, total latency, token usage, cost, structured-output failure rate and empty-retrieval rate
- [ ] **Alerts**: abnormal login activity, spikes in cross-workspace denials, queue backlog, sharply rising model error rate and abnormal per-user cost

### Complete the tests

- [ ] **Real PG integration tests**: transactions, permission filters, vector/full-text SQL (using a container with pgvector)
- [ ] **Queue tests**: duplicate delivery, Worker crash, timeout, retry and dead-letter recovery
- [ ] **Full E2E**: register → upload → analyze → create job → generate questions → simulate → report
- [ ] **Security tests**: horizontal privilege escalation, malicious files, expired signed URLs, Prompt Injection and log leakage
- [ ] **AI Golden Dataset evaluation set**: versioned; run regression before every Prompt/model upgrade; do not launch below the baseline

---

## P2 — Scaling and operations (complete before full production launch)

### Deployment and operations

- [ ] **Deploy pipeline**: add image build + push + deployment stages to CI
- [ ] **Production deployment architecture**: CDN/WAF → Ingress → multiple Web/API/Worker replicas → managed PG/Redis/object storage
- [ ] **KMS / Secret Manager**: keep secrets out of configuration files and manage production secrets centrally
- [ ] **Backup and recovery drills**: scheduled PG backups + recovery verification; object-storage lifecycle policies
- [ ] **Encryption at rest**: encrypt the database, object storage and backups

### Compliance

- [ ] **TLS end to end**
- [ ] **Log redaction**: email/phone/file contents/Token/Prompt evidence contents
- [ ] **Do not use user content for training/public retrieval by default**; sharing requires explicit authorization
- [ ] **Interview-content compliance**: compliant search APIs, respect robots and copyright; store only structured summaries + fingerprints + sources, not complete mirrored originals
- [ ] **AI scoring disclaimer**: label it as training advice, not a hiring conclusion

---

## Milestone comparison

| Plan milestone | Status | Remaining before launch |
| --- | --- | --- |
| M0 Baseline and contracts | ✅ Complete | — |
| M1 Resume and job | ✅ Complete | Replace with real resume-embedding pipeline |
| M2 Question set and text simulation | ✅ Complete (stub) | Connect question generation/follow-ups to a real model |
| M3 Report and quality gates | ✅ Complete (stub) | Connect scoring/reporting to a real model + AI evaluation-set gate |
| M4 Beta capabilities | ⚠️ Partial | Interview content/ASR have interfaces, but the intelligence layer is stubbed |

---

## Interview/presentation talking points

- **Solid already**: contract-driven API, JWT + Refresh rotation, presigned upload + async parsing, interview state machine, idempotent reports, tenant isolation and task dual-write
- **Still to build**: AI Orchestrator (`ChatModel`/`Embedding` interfaces), Prompt governance, model audit, cost governance and AI evaluation-set gates
- **One sentence**: the engineering foundation is demo-ready; the largest pre-launch engineering gap is the model-call, Prompt, retrieval, evaluation and cost-governance layer
