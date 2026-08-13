# AI 运行手册

模型调用失败时，先看用户任务状态，再看审计表和进程指标，不要直接重放用户简历或回答正文。

## 信号

| 信号 | 来源 | 阈值 |
| --- | --- | --- |
| Provider 错误率 | Worker 每 60 秒扫描 `model_invocations` 近 5 分钟 | 调用 ≥ 10 且失败率 ≥ 20% |
| 队列积压 | `async_tasks` pending 超过 5 分钟 | ≥ 10 |
| 任务卡住 | `async_tasks` running 超过 10 分钟 | ≥ 5 |
| 近 5 分钟失败任务 | `async_tasks` failed | ≥ 8 |
| 进程计数 | `GET /api/v1/metrics` | 失败、重试、预算拒绝、结构化修复失败上升 |

## 排查

1. 打开任务详情，确认 `error_code` 是 `AI_RATE_LIMITED`、`AI_TIMEOUT`、`AI_OUTPUT_INVALID`、`AI_BUDGET_EXHAUSTED` 还是 `AI_PROVIDER_UNAVAILABLE`。
2. 用 `task_id` / `session_id` 查 `model_invocations`：provider、model、prompt_key、prompt_version、tokens、latency、error_code。表中只有哈希，没有回答正文。
3. 看 API/Worker 日志。日志已脱敏，不应出现 API Key、密码、回答或简历全文。
4. 若结构化输出失败率升高，先回滚 Prompt 版本目录，而不是改业务表里的半成品 JSON。

## 降级

1. 将 `IM_AI_ENABLED=false` 并重启 API 与 Worker。题集和报告会进入 `degraded`，面试不再追问。
2. 需要降低成本时设置 `IM_AI_SMALL_MODEL`，软额度会切到小模型。
3. 追问超时或工具失败会自动进入下一道主问题，不要手工改 `interview_turns.ordinal`。

## 停用与恢复

1. 停用：关闭 AI 开关，暂停 `question:generate` / `report:generate` 队列，保留已完成面试。
2. 恢复：先用 Fake Model 单测和 Golden Dataset，再开小流量真实 Provider。
3. Prompt 回滚：保留旧版本目录（例如 `v1/`），把调用方的 version 指回去；不要覆盖正在用的文件。

## 数据保留

- `model_invocations` 默认保留 90 天，由 Worker 定时清理。
- 用户可通过设置页导出或删除账户。删除账户会级联业务数据，并尽力删除对象存储中的简历文件。
