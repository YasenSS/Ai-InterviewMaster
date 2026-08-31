# AI Operations Runbook

> Model failure troubleshooting and degradation. See the document map in [`../README.md`](../README.md); see [`../invite-beta-todo_0828.md`](../invite-beta-todo_0828.md) for the remaining invite-only work.

When a model call fails, check the user's task state first, then the audit table and process metrics. Do not directly replay the user's resume or answer body.

## Signals

| Signal | Source | Threshold |
| --- | --- | --- |
| Provider error rate | Worker scans the last 5 minutes of `model_invocations` every 60 seconds | At least 10 calls and a failure rate of at least 20% |
| Queue backlog | `async_tasks` pending for more than 5 minutes | ≥ 10 |
| Stuck tasks | `async_tasks` running for more than 10 minutes | ≥ 5 |
| Failed tasks in the last 5 minutes | `async_tasks` failed | ≥ 8 |
| Process counters | `GET /api/v1/metrics` | Failures, retries, budget rejections, or structured-output repair failures are increasing |

## Troubleshooting

1. Open the task details and confirm whether `error_code` is `AI_RATE_LIMITED`, `AI_TIMEOUT`, `AI_OUTPUT_INVALID`, `AI_BUDGET_EXHAUSTED`, or `AI_PROVIDER_UNAVAILABLE`.
2. Use `task_id` / `session_id` to query `model_invocations`: provider, model, prompt_key, prompt_version, tokens, latency, and error_code. The table contains hashes only, not the answer body.
3. Check API/Worker logs. Logs are redacted and must not contain API keys, passwords, answers, or the full resume text.
4. If structured-output failures increase, roll back the Prompt version directory first instead of editing half-finished JSON in business tables.

## Degradation

1. Set `IM_AI_ENABLED=false` and restart the API and Worker. Interview preparation and reports will use the local degradation strategy; dynamic follow-ups will safely switch to the next capability.
2. To reduce cost, set `IM_AI_SMALL_MODEL`; the soft quota will switch to the smaller model.
3. A follow-up timeout or tool failure will automatically move to the next main question. Do not manually edit `interview_turns.ordinal`.

## Disable and recover

1. Disable: turn off the AI flag, pause the `question:generate` / `report:generate` queues, and retain completed interviews.
2. Recover: run Fake Model unit tests and the Golden Dataset first, then enable a real Provider for a small amount of traffic.
3. Roll back a Prompt: retain the old version directory (for example, `v1/`) and point the caller's version back to it; do not overwrite files currently in use.

## Data retention

- `model_invocations` is retained for 90 days by default and cleaned up periodically by the Worker.
- Users can export or delete their accounts through the settings page. Account deletion cascades through business data and makes a best effort to delete resume files from object storage.
