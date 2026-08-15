# InterviewMaster Agent 架构与接口改造 TODO

> 日期：2026-08-14
> 设计依据：`agent设计.md`
> 当前基线：数据库 v9、现有 Interview API、`question:generate` / `report:generate` Worker
> 目标：把“固定候选题 + AI 追问插件”改造成“蓝图状态机 + 每轮实时生成下一问”

---

## 0. 文档定位

本文件是 Agent V2 的架构、接口、数据迁移和实施清单。

- `agent设计.md` 定义目标原则和产品行为。
- 本文件定义现有代码要怎样调整、接口怎样演进、任务怎样拆分。
- `agent_todo.md` 保留为历史基础设施清单，不再作为 Agent 主循环的验收依据。
- 本文件中的接口是目标契约；实现时必须同步更新 go-zero API、OpenAPI、生成客户端和前端调用。

### 状态标记

- `[ ]` 未开始。
- `[-]` 已开始但未完成验收。
- `[x]` 已完成且通过对应验收。
- `[!]` 被明确的外部依赖阻塞。

### 本轮不做

- 自主多 Agent。
- 通用长期记忆。
- 私有简历向量 RAG。
- 语音实时对话、TTS 或 WebSocket 全双工。
- 前端、客户端、Agent 开发等新岗位。
- 让用户管理内部题集、蓝图或异步任务。

---

## 1. 当前实现基线与主要问题

### 1.1 可以保留的基础

- 自研 `ChatModel` / `Tool` 抽象和 Eino Provider 适配。
- Prompt 版本化、JSON Schema、严格解码和一次修复。
- `model_invocations` 审计、Token、成本、限流和重试。
- PostgreSQL 会话状态、简历版本快照和完整 Turn 记录。
- Asynq Worker、任务认领、幂等和失败补偿基础。
- 简历事实工具、公司资料工具和最近三轮上下文组装。
- 面试记录、逐轮报告和分页页面。

### 1.2 必须替换的行为

| 当前行为 | 目标行为 |
| --- | --- |
| 蓝图和 Prompt 固定五题 | 蓝图保存最小、目标、最大轮数以及能力覆盖策略 |
| `questions` 是正常面试的播放队列 | `questions` 只保存锚点、Rubric 和降级兜底 |
| 只有 `follow_up` 实时生成问题 | 深挖和切换能力都必须实时生成下一问 |
| `next_capability` 从候选题中取题 | Agent 返回能力键和完整具体问题 |
| 模型可直接 `finish` | 模型只能 `recommend_finish`，状态机决定结束 |
| 回答接口同步等待模型 | 回答先持久化，进入可恢复的 `deciding`，异步生成下一问 |
| 回答后中断依赖 15 秒超时猜测恢复 | 使用持久化 decision 状态和唯一键恢复 |
| 动态 Turn 缺少评分字段 | 每个真实 Turn 保存完整问题快照 |
| 简历工具按用户搜索所有版本 | 工具固定绑定当前 `resume_version_id` |
| AI 关闭时静默表现为正常面试 | 明确返回并展示 `rule` 演示模式 |
| 通用 `/tasks/:id` 暴露给业务前端 | 改为面试域内的准备、决策、报告重试接口 |

---

## 2. 目标架构

### 2.1 组件职责

~~~text
Web
  ├─ 创建面试 / 查看准备状态
  ├─ 提交回答 / 展示 deciding
  ├─ 轮询当前 Session（MVP）
  └─ 查看面试记录与报告

API
  ├─ 鉴权和租户隔离
  ├─ 创建 Session 与内部蓝图载体
  ├─ 原子保存回答、phase 和 decision
  ├─ 查询面试聚合状态
  ├─ 用户主动结束
  └─ 业务化重试入口

Worker
  ├─ interview:prepare
  ├─ interview:next_turn
  └─ report:generate

AI Workflow
  ├─ ResumeExtraction
  ├─ BlueprintV2
  ├─ InterviewMaterials
  ├─ NextTurnDecisionV2
  ├─ TurnEvaluationV2
  └─ ReportCompose

PostgreSQL
  ├─ Session 策略与 phase
  ├─ Capability Progress
  ├─ Turn 不可变问题快照
  ├─ Next-turn Decision 幂等状态
  └─ Model Invocation 审计
~~~

### 2.2 主链路

~~~text
POST /interviews
  → 创建 preparing Session
  → 创建 interview:prepare 任务
  → Worker 生成 BlueprintV2、内部素材和第一题
  → Session = active / answering

PUT /interviews/:id/turns/:ordinal/answer
  → 原子保存答案
  → Session.phase = deciding
  → 创建唯一 next-turn decision
  → 投递 interview:next_turn
  → 立即返回 202

Next-turn Worker
  → 读取蓝图、能力进度、当前回答和最近三轮
  → 最多两次只读工具调用
  → 输出 NextTurnDecisionV2
  → 服务端策略判断结束或生成下一题
  → 原子更新 progress、Turn 和 phase

GET /interviews/:id
  → 前端短轮询
  → phase 变为 answering 后展示下一题
~~~

### 2.3 分层边界

- `backend/internal/platform/ai`：Provider、Eino、ReAct、结构化输出、审计和工具适配。
- `backend/internal/aiworkflow`：Blueprint、Materials、NextTurn、Evaluation、Report 等强类型节点。
- `backend/apps/api/internal/logic/workspace`：业务事务、状态机、接口错误和任务投递。
- `backend/apps/worker/internal/tasks`：可恢复异步执行，不直接拼装 HTTP 响应。
- Prompt 不访问数据库；工具调用由服务端绑定租户和简历版本。

---

## 3. 状态模型调整

### 3.1 Session 公共状态

继续使用：

~~~text
preparing
active
completed
failed
abandoned
~~~

语义：

- `preparing`：蓝图、内部素材或第一题尚未完成。
- `active`：面试可继续；具体阶段由 `phase` 表示。
- `completed`：面试问答结束；报告可以仍在生成。
- `failed`：仅用于准备阶段无法恢复的失败。
- `abandoned`：用户明确放弃且不要求生成完整报告。

### 3.2 Active 内部阶段

新增 `phase`：

~~~text
preparing
answering
deciding
decision_failed
completed
~~~

约束：

- `status=active` 时，phase 只能是 `answering`、`deciding` 或 `decision_failed`。
- `phase=answering` 必须存在且只存在一个未回答的当前 Turn。
- `phase=deciding` 必须存在一个已回答 Turn 和对应 pending/running decision。
- `phase=decision_failed` 必须保留答案、错误摘要和可重试 decision。
- `status=completed` 时，phase 固定为 `completed`。

### 3.3 状态转换

~~~text
preparing/preparing
  ├─ 准备成功 → active/answering
  └─ 准备失败 → failed/preparing

active/answering
  ├─ 提交答案 → active/deciding
  ├─ 跳过 → active/deciding
  └─ 用户结束 → completed/completed

active/deciding
  ├─ 生成下一题 → active/answering
  ├─ 状态机结束 → completed/completed
  └─ 决策失败 → active/decision_failed

active/decision_failed
  ├─ 重试 → active/deciding
  ├─ 明确使用兜底题 → active/answering
  └─ 用户结束 → completed/completed
~~~

---

## 4. 领域契约调整

### 4.1 `InterviewBlueprintV2`

删除固定 `question_count=5` 语义，改成：

~~~json
{
  "schema_version": "v2",
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
      "label": "项目深挖",
      "weight": 25,
      "target_evidence": 2,
      "difficulty_curve": ["medium", "hard"],
      "rubric": ["个人贡献", "技术取舍", "量化结果"]
    }
  ],
  "evidence_scope": ["resume", "answer", "company_intel"]
}
~~~

服务端校验：

- `min_turns <= target_turns <= max_turns`。
- 标准模式默认 `8 / 12 / 16 / 30 分钟`。
- 能力权重总和为 100。
- 每个能力键唯一。
- `max_follow_up_depth <= 2`。
- `max_follow_ups_total < max_turns`。
- Prompt 不能扩大服务端允许的上下限。

### 4.2 `InterviewMaterials`

替换当前“恰好五道题”的 `GeneratedQuestionSet`：

~~~json
{
  "capabilities": [
    {
      "capability_key": "project_depth",
      "expected_evidence": ["个人职责", "技术决策", "结果指标"],
      "anchor_questions": ["选择一个最具挑战性的项目说明你的个人贡献。"],
      "fallback_questions": ["这个项目里最关键的技术取舍是什么？"]
    }
  ]
}
~~~

规则：

- 锚点和兜底题数量由能力数决定，不作为真实面试总轮数。
- 材料相关内容必须引用当前简历版本的事实 ID。
- 正常 AI 路径不能按 `ordinal` 顺序播放这些问题。

### 4.3 `NextTurnDecisionV2`

~~~json
{
  "action": "deepen",
  "question": "你提到吞吐量提升了 40%，这个指标的基线和统计窗口是什么？",
  "turn_kind": "follow_up",
  "capability_key": "project_depth",
  "intent": "核实量化成果",
  "expected_points": ["基线", "统计窗口", "个人贡献"],
  "difficulty": "hard",
  "evidence_fact_ids": ["..."],
  "reason": "当前回答给出了结果但缺少指标口径",
  "coverage_observation": {
    "evidence_quality": 55,
    "resolved": [],
    "unresolved": ["指标口径"]
  }
}
~~~

动作只允许：

~~~text
deepen
switch_capability
recommend_finish
~~~

校验：

- `deepen` 和 `switch_capability` 的 `question` 必须非空。
- `recommend_finish` 不得包含下一题。
- `deepen` 不能越过当前能力，且不得超过追问深度和预算。
- `switch_capability` 必须选择蓝图中的能力键。
- 证据 ID 必须属于当前 Session 的 `resume_version_id`。
- `coverage_observation` 只能更新当前能力，数值由服务端裁剪。
- 模型不能直接修改 Session、轮数、时间预算或最终分数。

### 4.4 `CapabilityProgress`

~~~json
{
  "project_depth": {
    "asked_turns": 2,
    "follow_up_turns": 1,
    "evidence_count": 1,
    "evidence_quality": 55,
    "coverage_score": 50,
    "last_difficulty": "hard",
    "unresolved_gaps": ["指标口径"]
  }
}
~~~

更新策略：

- asked/follow-up 数由数据库真实 Turn 计算或服务端递增。
- 模型只能建议 evidence quality、resolved 和 unresolved。
- coverage score 使用确定性公式计算。
- 高权重能力覆盖不足时优先切换或继续深挖。

---

## 5. 公共 API 调整

### 5.1 接口变更总览

| 当前接口 | 调整 | 目标 |
| --- | --- | --- |
| `POST /interviews` | 保留，收窄请求并返回 preparing Session | 创建一次 AI 面试 |
| `GET /interviews` | 保留，增加 agent mode / phase / turn count | 面试记录列表 |
| `GET /interviews/:id` | 保留，增加策略、进度和操作状态 | 唯一轮询入口 |
| `PUT /interviews/:id/turns/:ordinal/answer` | 改为 202 异步推进 | 保存答案并触发下一问 |
| `POST /interviews/:id/turns/:ordinal/skip` | 改为 202 异步推进 | 跳过并触发下一问 |
| `POST /interviews/:id/complete` | 保留，明确为用户主动结束 | 不用于模型结束 |
| `POST /interviews/:id/answer` | 删除弃用接口 | 避免两套回答语义 |
| `GET /interviews/:id/report` | 保留，增强动态 Turn 字段 | 获取报告 |
| `GET /tasks/:id` | 迁移后删除公开接口 | 不暴露通用任务 |
| `POST /tasks/:id/retry` | 迁移后删除公开接口 | 使用业务化重试 |
| 无 | 新增准备重试 | `POST /interviews/:id/preparation/retry` |
| 无 | 新增下一轮重试 | `POST /interviews/:id/next-turn/retry` |
| 无 | 新增报告重试 | `POST /interviews/:id/report/retry` |
| 无 | P1 可选 SSE | `GET /interviews/:id/events` |

### 5.2 创建面试

#### `POST /api/v1/interviews`

请求：

~~~json
{
  "resume_id": "uuid",
  "primary_language": "Java",
  "target_company": "目标公司"
}
~~~

调整：

- 首版不再让用户传 `question_duration_seconds`。
- 岗位由后端固定为 `backend_development`。
- 模式默认 `standard`，以后增加模式时再作为可选字段。
- AI 未启用且非显式开发演示环境时返回 `AI_NOT_CONFIGURED`。

响应：`202 Accepted`

~~~json
{
  "id": "uuid",
  "status": "preparing",
  "phase": "preparing",
  "agent_mode": "ai",
  "primary_language": "Java",
  "target_company": "目标公司",
  "target_role": "backend_development",
  "policy": {
    "min_turns": 8,
    "target_turns": 12,
    "max_turns": 16,
    "time_budget_minutes": 30
  },
  "operation": {
    "type": "preparation",
    "status": "pending",
    "retryable": false
  },
  "current_turn": null,
  "created_at": "RFC3339"
}
~~~

不向前端返回 `question_set_id` 或通用 `task_id`。

### 5.3 获取面试

#### `GET /api/v1/interviews/:id`

建议响应：

~~~json
{
  "id": "uuid",
  "status": "active",
  "phase": "deciding",
  "agent_mode": "ai",
  "primary_language": "Java",
  "target_company": "目标公司",
  "target_role": "backend_development",
  "turn_count": 6,
  "answered_count": 6,
  "skipped_count": 0,
  "current_ordinal": 6,
  "elapsed_seconds": 720,
  "policy": {
    "min_turns": 8,
    "target_turns": 12,
    "max_turns": 16,
    "time_budget_minutes": 30
  },
  "progress": {
    "covered_weight": 45,
    "follow_ups_used": 2,
    "follow_ups_remaining": 2,
    "capabilities": []
  },
  "operation": {
    "type": "next_turn",
    "status": "running",
    "retryable": false,
    "error_code": "",
    "error_summary": ""
  },
  "current_turn": null,
  "turns": []
}
~~~

兼容策略：

- 新增 `turn_count`，替代语义含混的 `question_count`。
- 前端迁移完成后删除 `question_count`。
- `phase=deciding` 时 `current_turn` 可以为空，但 operation 必须说明正在生成。
- Turn 列表只返回真实提出过的问题。

### 5.4 提交回答

#### `PUT /api/v1/interviews/:id/turns/:ordinal/answer`

请求：

~~~json
{
  "answer": "用户回答",
  "client_request_id": "uuid"
}
~~~

响应：`202 Accepted`

~~~json
{
  "session_id": "uuid",
  "accepted_ordinal": 6,
  "phase": "deciding",
  "decision_status": "pending",
  "updated_at": "RFC3339"
}
~~~

约束：

- `client_request_id` 或 `Idempotency-Key` 必须唯一。
- 同一 Turn 的相同答案重复请求返回相同 decision 状态。
- 同一 Turn 的不同答案返回 `INTERVIEW_TURN_ALREADY_ANSWERED`。
- 回答、Session phase 和 decision 必须在同一事务中提交。
- API 不在请求生命周期内等待模型。

### 5.5 跳过问题

#### `POST /api/v1/interviews/:id/turns/:ordinal/skip`

- 保存 `skipped_at`。
- 创建 next-turn decision。
- 返回与提交回答相同的 `202` 结构。
- Agent 输入明确标记本题跳过，不把空答案判断为能力不足。

### 5.6 用户主动结束

#### `POST /api/v1/interviews/:id/complete`

请求：

~~~json
{
  "confirm_incomplete": true
}
~~~

规则：

- 这是用户主动结束，不是模型结束接口。
- `phase=deciding` 时默认返回冲突，避免正在生成的决策和完成事务竞争。
- 用户二次确认后可以取消 pending decision，再结束面试。
- 保存 `completion_reason=user_ended`。
- 已完成 Session 幂等返回当前状态。

### 5.7 业务化重试

#### `POST /api/v1/interviews/:id/preparation/retry`

- 只允许 `status=failed` 且最新准备任务失败。
- 创建新的准备 attempt，并恢复 `preparing`。

#### `POST /api/v1/interviews/:id/next-turn/retry`

- 只允许 `status=active && phase=decision_failed`。
- 重用原 answered turn 和 decision ID，增加 attempt。
- 不允许再次保存或修改回答。

#### `POST /api/v1/interviews/:id/report/retry`

- 只允许面试已完成且最新报告状态为 failed。
- 不向前端暴露通用任务类型。

### 5.8 P1 SSE

#### `GET /api/v1/interviews/:id/events`

事件建议：

~~~text
interview.prepared
decision.started
turn.ready
decision.failed
interview.completed
report.ready
report.failed
~~~

MVP 可以先通过 `GET /interviews/:id` 短轮询实现，不阻塞主循环改造。

### 5.9 错误码

新增或统一：

~~~text
AI_NOT_CONFIGURED
INTERVIEW_NOT_ACTIVE
INTERVIEW_NOT_ANSWERING
INTERVIEW_DECISION_IN_PROGRESS
INTERVIEW_DECISION_FAILED
INTERVIEW_TURN_ALREADY_ANSWERED
INTERVIEW_TURN_NOT_CURRENT
INTERVIEW_POLICY_REJECTED
NEXT_TURN_OUTPUT_INVALID
NEXT_TURN_PROVIDER_UNAVAILABLE
NEXT_TURN_BUDGET_EXHAUSTED
NEXT_TURN_RETRY_NOT_ALLOWED
~~~

客户端只能获得安全摘要，Provider 原始错误保留在服务端审计中。

---

## 6. 数据库迁移 TODO

新增 `00010_agent_runtime_v2.sql`。

### DB-001 Session 字段

- [ ] 增加 `agent_mode text NOT NULL`，限定 `ai | rule | legacy`。
- [ ] 增加 `phase text NOT NULL`。
- [ ] 增加 `policy_version text`。
- [ ] 增加 `interviewer_prompt_version text`。
- [ ] 增加 `min_turns`、`target_turns`、`max_turns`。
- [ ] 增加 `time_budget_minutes`。
- [ ] 增加 `max_follow_up_depth`、`max_follow_ups_total`。
- [ ] 增加 `capability_progress jsonb NOT NULL DEFAULT '{}'`。
- [ ] 增加 `completion_reason text`。
- [ ] 增加 phase/status CHECK 约束。
- [ ] 为 `user_id + status + phase + updated_at` 建索引。

### DB-002 Turn 快照字段

- [ ] 增加 `intent text NOT NULL DEFAULT ''`。
- [ ] 增加 `expected_points jsonb NOT NULL DEFAULT '[]'`。
- [ ] 增加 `difficulty text NOT NULL DEFAULT 'medium'`。
- [ ] 增加 `evidence_fact_ids jsonb NOT NULL DEFAULT '[]'`。
- [ ] 增加 `decision_reason text NOT NULL DEFAULT ''`。
- [ ] 增加 `coverage_observation jsonb NOT NULL DEFAULT '{}'`。
- [ ] 将 `invocation_id` 补上对 `model_invocations(id)` 的可选外键。
- [ ] 保留 `source_question_id`，但只用于锚点或兜底题。

### DB-003 Next-turn Decision 表

- [ ] 新建 `interview_turn_decisions`：

~~~text
id uuid PK
session_id uuid FK
answered_turn_id uuid FK
next_turn_id uuid FK nullable
status pending/running/succeeded/failed/cancelled
action
attempt
input_hash
model_invocation_id uuid nullable
error_code
error_summary
created_at
started_at
completed_at
updated_at
~~~

- [ ] `answered_turn_id` 建唯一约束，保证一份答案只产生一个逻辑决策。
- [ ] pending/running 状态建立恢复扫描索引。
- [ ] 所有更新同时校验 Session 所属用户和当前 phase。

### DB-004 内部素材兼容

- [ ] `question_sets.blueprint` 升级为 v2，保留 Schema version。
- [ ] `questions` 增加 `material_kind=anchor|fallback` 或等价字段。
- [ ] 不再要求题数恰好为 5。
- [ ] 不再依靠 `questions.ordinal` 决定真实面试顺序。

### DB-005 历史数据迁移

- [ ] 已完成 Session 标记为 `agent_mode=legacy`，保持记录可读。
- [ ] 旧 active Session 继续走 legacy runner，或在发布前明确封存；不能半途切换 V2。
- [ ] 新建 Session 一律使用 v2 policy。
- [ ] v10 Down 明确处理 V2 active Session，不允许静默丢失 decision。
- [ ] 用 v9 历史样本完成 up/down/up 验证。

---

## 7. 内部任务与 Worker TODO

### TASK-001 重命名准备任务

- [ ] 将 `question:generate` 的业务语义改为 `interview:prepare`。
- [ ] 迁移期间 Worker 同时注册旧、新任务名，避免队列中旧消息丢失。
- [ ] Payload 保留 Session、User、Resume、ResumeVersion。
- [ ] Worker 只从数据库读取语言、公司、岗位和 policy，减少 payload 漂移。
- [ ] 成功产物为 BlueprintV2、Materials 和第一条 Turn。
- [ ] AI 模式准备失败时进入 failed，不自动伪造“AI 成功”。

### TASK-002 新增下一轮任务

- [ ] 新增 `TypeInterviewNextTurn = "interview:next_turn"`。
- [ ] Payload 最小化：

~~~json
{
  "decision_id": "uuid",
  "session_id": "uuid",
  "answered_turn_id": "uuid",
  "user_id": "uuid"
}
~~~

- [ ] Worker claim 时校验用户、Session、Turn、decision 和 phase 关联。
- [ ] 已 succeeded/cancelled 任务直接幂等返回。
- [ ] running 超时允许受控接管。
- [ ] 模型调用发生在数据库事务之外。
- [ ] 应用决策时重新锁 Session 并校验状态版本。
- [ ] 下一 Turn 和 progress 更新在同一事务提交。
- [ ] Ack 前崩溃重放不得重复插入 Turn。

### TASK-003 决策失败策略

- [ ] 明确区分可重试 Provider 错误、输出错误、预算错误和永久配置错误。
- [ ] 自动重试次数受限，不得无限消耗模型。
- [ ] 最终失败后 Session 进入 `decision_failed`。
- [ ] 前端显示安全错误摘要和重试入口。
- [ ] 只有用户选择“使用兜底继续”时才插入 fallback Turn。
- [ ] fallback Turn 标记来源和 `agent_mode/rule`，不能伪装成实时 AI。

### TASK-004 恢复扫描

- [ ] Worker 定期扫描超时 preparing task。
- [ ] Worker 定期扫描超时 pending/running decision。
- [ ] 恢复动作使用相同逻辑唯一键。
- [ ] 恢复次数超过阈值后进入明确失败状态。

### TASK-005 报告任务

- [ ] 报告直接读取 Turn 快照。
- [ ] 动态主问题和追问采用相同评分路径。
- [ ] 报告任务不再通过 `source_question_id` 获取必要 Rubric。
- [ ] 对正式评分做受控并行，避免长面试串行超时。

---

## 8. Agent 与 Prompt 实现 TODO

### AI-001 Blueprint V2

- [ ] 新增 `blueprint.generate/v2`，不修改 v1。
- [ ] 移除固定五题提示。
- [ ] 输出 min/target/max、能力权重、证据目标和难度曲线。
- [ ] Go Validator 强制服务端边界。
- [ ] 为非法轮数、权重、重复能力和越界预算补测试。

### AI-002 Materials

- [ ] 新增 `interview.materials/v1`。
- [ ] 输出 Rubric、期望证据、anchor 和 fallback。
- [ ] 增加去重、能力覆盖和事实 ID 校验。
- [ ] 正常流程不得把 Materials 批量复制到 Turns。

### AI-003 Next-turn V2

- [ ] 新增 `interview.next_turn/v2`。
- [ ] 实现 `NextTurnInputV2` 和 `NextTurnDecisionV2`。
- [ ] `deepen` / `switch_capability` 强制返回完整问题快照。
- [ ] `recommend_finish` 只作为策略输入。
- [ ] 上下文包含 policy、progress、elapsed time、当前问答和最近三轮。
- [ ] 不再查询候选题作为正常 `next_capability`。

### AI-004 单次 ReAct 运行

- [ ] 重构当前 `RunToolLoop + StructuredRunner` 双调用。
- [ ] 无工具时直接接受同一次模型的结构化输出。
- [ ] 有工具时把结果回灌，最终一次输出结构化决策。
- [ ] 每轮工具调用不超过 2 次。
- [ ] Deadline 覆盖整个逻辑运行，且与 Provider 单次超时配置协调。

### AI-005 工具范围

- [ ] `ResumeFactsTool` 构造参数改为 `resume_version_id`，同时保留 user guard。
- [ ] 查询 SQL 必须同时匹配 user、resume 和 version。
- [ ] Company Intel 限制公司、岗位和返回条数。
- [ ] 未命中资料时返回空结果，不生成虚构事实。

### AI-006 服务端策略

- [ ] 新建纯 Go `InterviewPolicy`。
- [ ] 实现 max turns、time budget、min turns 和 coverage 判断。
- [ ] 实现高权重能力覆盖不足判断。
- [ ] 实现追问深度和整场追问预算。
- [ ] 模型输出不能绕过状态机。
- [ ] Policy 使用版本号并写入 Session。

### AI-007 能力进度

- [ ] 设计确定性 coverage 公式。
- [ ] 将模型 observation 限制为当前能力。
- [ ] Turn 提交后更新 asked/follow-up 计数。
- [ ] evidence quality 和 unresolved gaps 进入下一轮上下文。
- [ ] 为重复更新和并发更新增加测试。

---

## 9. API 与生成代码 TODO

### API-001 修改 go-zero 契约

- [ ] 更新 `backend/api/interviewmaster.api`。
- [ ] 删除 `AnswerInterviewRequest` 及弃用路由。
- [ ] 调整 Create、Session、Turn、AnswerAccepted 类型。
- [ ] 增加 Policy、Progress、Operation 类型。
- [ ] 增加三个业务化 retry 路由。
- [ ] 迁移完成后删除公开 TaskPath/GetTask/RetryTask。

### API-002 更新生成物

- [ ] 重新生成 routes、handlers 和 types。
- [ ] 重新生成 `backend/api/openapi/interviewmaster.yaml`。
- [ ] 重新生成 Web API Client 和 Components。
- [ ] 确认生成物中不存在 JD、公开题集和公开通用任务接口。
- [ ] `goctl api validate` 通过。

### API-003 逻辑层

- [ ] Create Interview 返回准备操作，不返回 task ID。
- [ ] Get Interview 聚合 phase、policy、progress、current turn 和 operation。
- [ ] Answer/Skip 只负责持久化和投递，不同步调用模型。
- [ ] Complete 与 pending decision 做并发互斥。
- [ ] Retry 只认最新失败操作。
- [ ] 所有写操作继续校验 `session.user_id`。

### API-004 并发与幂等

- [ ] 两个并发回答只有一个成功。
- [ ] 相同 `client_request_id` 返回相同结果。
- [ ] 已保存答案不能被覆盖。
- [ ] 一个 answered Turn 只能关联一个 decision。
- [ ] next-turn apply 使用状态版本或严格行锁。

---

## 10. 前端 TODO

### WEB-001 创建页

- [ ] 保持“简历 + 语言 + 公司”三个核心输入。
- [ ] 移除 `question_duration_seconds`。
- [ ] 默认标准 30 分钟，不新增必须选择的复杂配置。
- [ ] AI 未配置时禁止以 AI 模式开始，开发演示模式明确标识。

### WEB-002 面试房间

- [ ] 读取并展示 `phase`。
- [ ] `answering` 显示当前问题和回答输入。
- [ ] `deciding` 显示“AI 面试官正在分析你的回答”。
- [ ] `decision_failed` 显示错误摘要、重试和使用兜底继续。
- [ ] 提交回答收到 202 后立即清理草稿并开始轮询。
- [ ] 不在前端假设问题总数为 5。
- [ ] 显示“已完成 N 轮”，目标轮数只做范围提示。

### WEB-003 模式与降级

- [ ] 面试页显示 `AI 面试` 或 `规则演示`。
- [ ] fallback Turn 显示“系统兜底问题”，不能标成基于回答的追问。
- [ ] 记录页保留真实 turn kind 和生成模式。

### WEB-004 重试去任务化

- [ ] 准备失败调用 preparation retry。
- [ ] 下一问失败调用 next-turn retry。
- [ ] 报告失败调用 report retry。
- [ ] 删除 Web 对通用 `tasks/:id` 的依赖。

### WEB-005 SSE（P1）

- [ ] 轮询稳定后再增加 EventSource。
- [ ] SSE 断开自动退回轮询。
- [ ] 事件只作为刷新信号，数据库查询仍是权威状态。

---

## 11. 报告与记录接口 TODO

### REPORT-001 Turn Report

- [ ] `TurnReportResponse` 增加：

~~~text
turn_kind
capability_key
intent
difficulty
expected_points
generation_mode
~~~

- [ ] 评分展示不依赖内部题集。
- [ ] 动态追问和动态主问题都必须有点评与参考答案。

### REPORT-002 Summary

- [ ] 列表使用 `turn_count` 而不是 `question_count`。
- [ ] `completed` 但报告 pending 时不显示 0 分。
- [ ] Dashboard 只聚合 completed/degraded 报告。

---

## 12. 测试与验收 TODO

### TEST-001 Contract

- [ ] Blueprint V2 合法/非法边界测试。
- [ ] NextTurnDecision V2 三种动作测试。
- [ ] 非结束动作缺 question 必须拒绝。
- [ ] 未知能力键、越权事实 ID、越界观察值必须拒绝。
- [ ] Policy finish 条件表驱动测试。

### TEST-002 Agent

- [ ] 回答包含具体指标时生成核实指标口径的追问。
- [ ] 当前能力证据充分时切换能力并生成新问题。
- [ ] 未达到 min turns 时拒绝 recommend finish。
- [ ] 达到 max turns 或时间预算时强制结束。
- [ ] 无工具调用时只发生一次最终模型请求。
- [ ] 工具调用最多两次。

### TEST-003 数据与并发

- [ ] 并发重复回答。
- [ ] Answer commit 后 Worker 崩溃恢复。
- [ ] Next Turn commit 后 Ack 前崩溃重放。
- [ ] 最新失败 decision 原地重试。
- [ ] 旧 decision 不能覆盖新状态。
- [ ] 跨用户、跨简历版本工具访问拒绝。

### TEST-004 API

- [ ] Create 返回 202 preparing。
- [ ] Answer/Skip 返回 202 deciding。
- [ ] Get Interview 从 deciding 变成 answering。
- [ ] Complete 与 deciding 冲突处理。
- [ ] 三类业务化 retry 状态约束。
- [ ] 公开 OpenAPI 不再包含通用 task 查询和重试。

### TEST-005 前端

- [ ] answering / deciding / decision_failed / completed 页面状态。
- [ ] AI 与规则模式标识。
- [ ] 超过五轮仍正常显示和提交。
- [ ] 刷新页面不会丢失 deciding 状态。
- [ ] 轮询或 SSE 能拿到新 Turn。

### TEST-006 真实模型闭环

- [ ] 配置真实 OpenAI-compatible Provider。
- [ ] 使用两份不同简历完成至少两场标准面试。
- [ ] 每场自然超过五轮。
- [ ] 下一问能够针对上一轮回答。
- [ ] `model_invocations` 每个已回答 Turn 都有对应记录。
- [ ] 完成准备、动态问答、状态机结束和报告闭环。
- [ ] 记录延迟、Token、成本、失败率和降级次数。

---

## 13. 发布与兼容 TODO

### RELEASE-001 双版本兼容

- [ ] 新 Session 使用 policy v2。
- [ ] 历史 completed Session 只读兼容。
- [ ] 历史 active Session 明确走 legacy 或封存，不能动态混用。
- [ ] Worker 在过渡期兼容旧准备任务名。

### RELEASE-002 灰度

- [ ] 增加服务端 `agent_runtime_v2` 灰度开关。
- [ ] 先对内部账号启用。
- [ ] 监控 next-turn 成功率、P95、平均工具次数和单场成本。
- [ ] 失败率超过阈值时停止创建新 V2 Session，不修改历史 Session。

### RELEASE-003 回滚

- [ ] 发布前备份数据库。
- [ ] 回滚应用时保持 V2 表和数据可读。
- [ ] 不把已开始的 V2 Session 回退成固定题单。
- [ ] 必要时停止新面试，允许已完成记录和报告继续访问。

---

## 14. 推荐实施批次

### Batch A：契约与数据库

- [ ] AI-001 Blueprint V2。
- [ ] AI-002 Materials。
- [ ] AI-003 Next-turn V2 Contract。
- [ ] DB-001～DB-005。
- [ ] TEST-001。

完成标准：迁移可回滚，所有领域契约通过测试，但暂不切换现有流量。

### Batch B：下一轮 Worker 与状态机

- [ ] TASK-002～TASK-004。
- [ ] AI-004～AI-007。
- [ ] API-003～API-004。
- [ ] TEST-002～TEST-003。

完成标准：Fake Model 下可以稳定完成超过五轮的动态面试，并能恢复崩溃。

### Batch C：公共接口与前端

- [ ] API-001～API-002。
- [ ] WEB-001～WEB-004。
- [ ] TEST-004～TEST-005。

完成标准：用户提交后看到 deciding，随后出现基于回答生成的新问题。

### Batch D：报告和真实模型

- [ ] TASK-005。
- [ ] REPORT-001～REPORT-002。
- [ ] TEST-006。

完成标准：真实 Provider 完成准备、至少八轮动态问答、结束和完整评分报告。

### Batch E：灰度与清理

- [ ] RELEASE-001～RELEASE-003。
- [ ] 删除 legacy 回答接口。
- [ ] 删除公开通用 Task API。
- [ ] 删除固定五题 Prompt 和正常播放路径。
- [ ] P1 再评估 SSE。

---

## 15. 文件级调整索引

| 文件/目录 | 主要调整 |
| --- | --- |
| `backend/api/interviewmaster.api` | 新响应类型、202 Answer、业务重试、删除旧接口 |
| `backend/api/openapi/interviewmaster.yaml` | 重新生成 |
| `backend/internal/platform/ai/contract/contract.go` | BlueprintV2、Materials、NextTurnDecisionV2、Policy |
| `backend/internal/platform/ai/react.go` | 单次 ReAct 最终结构化输出 |
| `backend/internal/platform/ai/tools/tools.go` | 绑定 resume_version_id |
| `backend/internal/aiworkflow/interviewer.go` | 改为 next-turn v2 |
| `backend/internal/aiworkflow/workflow.go` | 拆分 blueprint/materials/evaluation/report |
| `backend/apps/api/internal/logic/workspace/createinterviewlogic.go` | phase、异步 Answer、业务重试 |
| `backend/apps/api/internal/logic/workspace/interviewer.go` | 状态机与 apply decision，删除候选题正常取题 |
| `backend/apps/worker/internal/tasks/questiongen.go` | 改为 interview prepare |
| `backend/apps/worker/internal/tasks/nextturn.go` | 新增 |
| `backend/apps/worker/internal/tasks/reportgen.go` | 使用 Turn 快照 |
| `backend/internal/tasks/question.go` | 准备任务重命名与兼容 |
| `backend/internal/tasks/nextturn.go` | 新增 payload |
| `backend/prompts/blueprint.generate/v2/` | 新增 |
| `backend/prompts/interview.materials/v1/` | 新增 |
| `backend/prompts/interview.next_turn/v2/` | 新增 |
| `backend/migrations/00010_agent_runtime_v2.sql` | Session、Turn、Decision |
| `web/src/shared/api/*` | 重新生成并调整服务封装 |
| `web/src/features/interviews/InterviewRoomPage.tsx` | phase、deciding、重试、模式标识 |
| `web/src/features/interviews/InterviewRecordPage.tsx` | 动态问题快照和生成模式 |

---

## 16. Definition of Done

只有同时满足以下条件，Agent V2 才能标记完成：

- [ ] 正常路径不再从固定候选题队列取下一题。
- [ ] 每次回答后由真实模型生成具体下一问。
- [ ] 标准面试最少 8 轮，可自然达到目标 12 轮，最多不超过 16 轮。
- [ ] 状态机而不是模型决定结束。
- [ ] 回答提交、deciding、失败重试和崩溃恢复全部持久化。
- [ ] 动态 Turn 保存完整评分快照。
- [ ] 简历工具严格绑定当前 Session 的简历版本。
- [ ] AI 关闭或降级在前端明确可见。
- [ ] 公共 API 不暴露题集和通用任务。
- [ ] 真实 Provider E2E 的 `model_invocations` 不为 0。
- [ ] 后端测试、前端测试、构建、迁移 up/down/up 和真实 HTTP E2E 全部通过。
- [ ] `agent设计.md`、OpenAPI、本文件和最终代码保持一致。
