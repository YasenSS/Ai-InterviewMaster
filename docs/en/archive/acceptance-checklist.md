# InterviewMaster Acceptance Checklist

> **Archived (2026-08-28).** Contains outdated JD and fixed-five-question flows. Invitation-only acceptance is defined in [`../invite-beta-todo_0828.md`](../invite-beta-todo_0828.md).

## M0

- `docker compose config --quiet` passes.
- API `/health` and `/ready` are accessible.

## M1

- Register/log in and obtain a JWT; data belonging to different users is mutually invisible.
- Creating a JD returns capability tags.
- PDF/DOCX/TXT files can be uploaded through a presigned URL; the Worker parses them through Tika and writes facts, chunks, and baseline pgvector embeddings.

## M2

- A parsed resume can generate a structured five-question set.
- A text interview can be created, answered one question at a time, resumed after refresh, and enter `completed` after the final question.

## M3

- A completed interview can generate overall and per-question scores, critiques, Golden Answers, evidence, and pass the quality gate.
- Repeated queries for the same session reuse the same report.

## Beta

- The interview-intelligence API provides local fallback results, and the external search provider can be replaced.
- `docker compose --profile beta up -d asr` starts local Whisper; the ASR API submits a task, and the Worker transcribes it. The result can be queried through `/api/v1/tasks/{id}`.

## Automated checks

```powershell
.\scripts\check.ps1
docker compose config --quiet
```

Most recent complete acceptance result: all 5 migration groups succeeded; both the resume-parsing task and the ASR task were `succeeded`; the text interview completed 5 rounds and generated a report that passed the quality gate.
