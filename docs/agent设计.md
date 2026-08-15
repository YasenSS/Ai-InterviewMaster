# InterviewMaster Agent 设计

> 初版日期：2026-08-12
> 修订日期：2026-08-14
> 定位：智能层（Agent / LLM / 编排）的目标设计与实现约束
> 产品边界：用户上传简历、选择主技术语言和目标公司；首版岗位固定为后端开发

---

## 0. 一句话结论

**不做自主多 Agent；采用“确定性业务工作流 + 一处收窄的实时面试官 ReAct 循环”。**

- 简历抽取、蓝图生成、最终评分和报告是已知步骤，由确定性工作流编排。
- 全系统唯一接近自主 Agent 的部分，是用户每次回答后运行的面试官节点。
- 面试官可以查只读资料、分析回答、选择深挖或切换能力，并生成下一道具体问题。
- 面试何时结束由服务端状态机决定，模型只能建议结束。
- 内部题集是蓝图、能力 Rubric、考察素材和故障兜底，不是按顺序播放的固定题单，也不能成为面试轮数上限。
- MVP 不需要多 Agent、跨会话长期记忆或通用上下文压缩。

---

## 1. 产品目标与边界

### 1.1 用户主流程

```text
上传并解析简历
  → 选择主技术语言
  → 选择目标公司
  → 点击开始面试（岗位固定为后端开发）
  → 后台生成面试蓝图、能力 Rubric、兜底素材和第一题
  → 用户回答
  → 面试官 Agent 根据回答实时生成下一问
  → 状态机根据轮数、时间和能力覆盖决定继续或结束
  → 异步生成逐轮评分与完整复盘
```

用户创建和感知的是“一次面试”，不是 JD、题集或通用异步任务。

### 1.2 首版固定约束

- 岗位固定为后端开发，不再依赖 JD 模块。
- 面试输入为：简历版本、主技术语言、目标公司、固定岗位画像。
- 默认使用标准面试模式，不要求用户额外配置复杂参数。
- 标准模式默认策略：
  - 最少 8 轮；
  - 目标约 12 轮；
  - 最多 16 轮；
  - 时间预算 30 分钟；
  - 单条主问题链最多连续追问 2 次；
  - 整场追问预算默认 4 次。
- 以上数字属于服务端安全边界和默认策略，未来可以扩展快速、标准、深度模式，不能再次散落硬编码到 Prompt、Schema 和业务逻辑中。

### 1.3 AI 运行模式必须真实可见

- `AI.Enabled=true`：真实 AI 面试模式，每轮正常路径都必须产生可审计的模型调用。
- `AI.Enabled=false`：规则演示模式，只用于本地开发或故障演练。
- 规则模式必须在前端明确提示，不能让用户误以为正在接受 AI 面试。
- 生产环境若未配置模型，应启动失败或禁止创建 AI 面试，不能静默伪装成正常模式。

---

## 2. 总体架构：专业化节点，而非多 Agent

### 2.1 节点划分

| 节点 | 输入 | 输出 | 编排方式 |
| --- | --- | --- | --- |
| 简历事实抽取 | Tika 文本 | 技能、项目、指标、原文证据 | Chain |
| 面试蓝图生成 | 简历事实 + 语言 + 公司 + 固定岗位画像 | 能力计划、轮数范围、难度曲线、时间预算 | Chain |
| 内部素材生成 | 蓝图 + 简历事实 + 公司资料 | 能力 Rubric、期望证据、锚点题、兜底题 | Graph |
| 第一题生成 | 蓝图 + 当前能力 | 第一条真实面试问题及其评分快照 | Chain |
| **实时面试官** | 蓝图进度 + 最近问答 + 当前回答 | 深挖、切换能力或建议结束；非结束时必须包含下一题 | **收窄 ReAct** |
| 最终逐轮评估 | 完整真实问答 + 问题评分快照 + 简历证据 | 五维评分、点评、参考答案 | Workflow |
| 报告撰写 | 全部逐轮评估 + 能力覆盖 | 总评、强项、改进项、训练计划 | Chain |

### 2.2 框架边界

- 继续使用 Eino，但 Eino 只能存在于 `backend/internal/platform/ai/` 内部。
- 业务层只依赖自研的 `ChatModel`、`Tool`、`InterviewerAgent` 等接口。
- 领域逻辑不得直接 import Eino，保证未来可以更换 Provider 或编排实现。
- Eino Graph 用于有分支的生成、校验、去重和覆盖检查。
- Eino Workflow 用于字段映射清晰的多阶段评分。
- Eino ADK 或等价的内部实现只服务于实时面试官 ReAct。
- 是否使用某个框架不是验收目标；“每轮真实调用模型并生成下一问”才是产品验收目标。

---

## 3. 面试蓝图与内部题集

### 3.1 蓝图是可执行策略

蓝图不能只是存档 JSON，必须直接驱动状态机。建议结构：

```json
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
      "rubric": ["个人贡献", "技术取舍", "量化结果"]
    }
  ]
}
```

蓝图必须控制：

- 最小、目标和最大总轮数；
- 高权重能力优先级；
- 每项能力需要取得的有效证据数量；
- 当前问题难度和下一步难度；
- 单链追问深度与整场追问预算；
- 时间预算和最终结束条件。

不再保留“所有面试固定恰好 5 题”的规则。轮数范围只能从蓝图策略读取。

### 3.2 内部题集的正确职责

内部 `question_set` 可以继续存在，但只保存：

- 面试蓝图；
- 能力 Rubric；
- 每项能力的期望回答证据；
- 少量锚点问题；
- 模型不可用时的兜底问题；
- Prompt、模型和输入快照信息。

内部题目不能：

- 作为正常面试按顺序播放的队列；
- 决定面试只能进行多少轮；
- 提前复制成所有公开面试轮次；
- 被前端作为用户可管理资源展示。

正常路径下，真实提出的问题必须由实时面试官生成并写入 `interview_turns`。只有降级路径才允许使用内部兜底问题。

---

## 4. 实时面试官主循环

### 4.1 状态模型

```text
preparing
  → active(answering)
  → active(deciding)
  → active(answering)
  → ...
  → completed

preparing/deciding 失败
  → 可重试
  → 明确降级或失败
```

`active` 是公开业务状态，`answering` / `deciding` 可以作为内部 phase。回答已经持久化但下一问尚未生成时，必须处于可恢复的 `deciding` 状态，不能依靠超时猜测恢复。

### 4.2 主循环

```text
prepare:
    锁定 resume_version_id
    blueprint = 生成可执行蓝图
    materials = 生成 Rubric、锚点和兜底素材
    first_question = 面试官生成第一题
    保存第一题完整快照

while session.status == active:
    保存用户回答
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
        完成面试
    else:
        校验 decision 中的下一题
        更新能力进度
        保存下一题完整快照
        session.phase = answering
```

### 4.3 Agent 决策契约

建议将决策动作收敛为：

```text
deepen             围绕当前回答继续深挖
switch_capability  切换到覆盖不足的能力
recommend_finish   模型认为继续提问收益很低
```

`deepen` 和 `switch_capability` 都必须返回一条具体的下一问，并包含：

- `question`
- `turn_kind`
- `capability_key`
- `intent`
- `expected_points`
- `difficulty`
- `evidence_fact_ids`
- `reason`
- `coverage_observation`

`recommend_finish` 只是一项建议，模型不能直接修改 Session 状态。

### 4.4 结束权归状态机

服务端结束策略按以下顺序判断：

1. 达到 `max_turns`：强制结束。
2. 达到时间预算：完成当前轮后结束。
3. 未达到 `min_turns`：禁止结束。
4. 高权重能力尚未覆盖：禁止结束。
5. 达到 `target_turns` 且整体证据覆盖满足阈值：允许结束。
6. 模型建议结束且覆盖条件满足：允许提前于 `target_turns` 结束，但不得低于 `min_turns`。

模型输出无效时不能改变这些边界。

### 4.5 一次逻辑运行只产出一次最终决策

每轮允许最多两次只读工具调用，但工具循环结束后的模型输出应直接成为结构化决策。

不能采用：

```text
先调用一次模型判断要不要用工具
  → 无论是否调用工具
  → 再调用一次模型生成相同决策
```

无工具调用时，一轮面试官通常只应有一次模型请求；有工具调用时，额外请求只能来自真实的工具回灌循环。

---

## 5. 工具调用

### 5.1 MVP 只读工具

| 工具 | 作用 | 数据边界 |
| --- | --- | --- |
| `lookup_resume_facts` | 查证候选人的项目、技能和指标 | 必须绑定当前 Session 的 `resume_version_id` |
| `lookup_company_intel` | 查询本地维护的目标公司和岗位考察资料 | 只读公共资料，并限制结果数量 |

后续可以增加公开面经检索，但不能让模型自行传入 `user_id`、租户 ID 或简历版本 ID。

### 5.2 ReAct 边界

- 每轮最多调用工具 2 次。
- 工具只读，不需要权限确认。
- 工具结果必须裁剪后回灌。
- 工具失败不应直接结束会话；模型可以在缺少工具结果时继续生成不捏造事实的问题。
- 工具调用、最终决策和对应 Turn 必须能够通过 invocation 记录关联。

---

## 6. 上下文、状态与能力进度

### 6.1 使用状态，不建设通用长期记忆

PostgreSQL 是面试 Agent 的可靠状态源：

- `interview_sessions` 保存蓝图、策略、当前 phase 和进度；
- `interview_turns` 保存每次真实提出的问题和用户回答；
- 能力进度保存每项能力已经取得的证据和覆盖程度；
- 每次模型调用前从数据库重新组装上下文。

不需要 Claude Code 式通用长期记忆，也不需要把完整历史永久塞入模型上下文。

### 6.2 每轮上下文

模型输入只包括：

- 系统规则；
- 固定岗位画像、主语言和目标公司；
- 当前蓝图和能力进度摘要；
- 当前问题与用户回答；
- 最近 2～3 轮问答；
- 已用轮数、追问数和剩余时间；
- 当前 Session 锁定的简历事实 ID；
- 必要时通过工具查询得到的证据。

最近问答采用裁剪，不做 LLM 上下文摘要。能力进度是结构化状态，不依赖模型记忆。

### 6.3 能力进度

每项能力至少记录：

```text
asked_turns
follow_up_turns
evidence_count
evidence_quality
coverage_score
last_difficulty
unresolved_gaps
```

模型可以给出 `coverage_observation`，但服务端必须校验并限制更新范围，不能让模型任意修改轮数、预算或其他能力的状态。

---

## 7. 评估与报告

### 7.1 在线轻量观察

为了决定下一问，面试官每轮需要产生轻量观察：

- 回答是否切题；
- 是否提供具体个人行动；
- 是否有可验证证据；
- 哪些信息仍然缺失；
- 当前能力是否值得继续深挖。

这部分与下一问放在同一次 Agent 最终输出中，避免为每轮额外增加一次独立评分调用。

### 7.2 面试结束后的正式评分

正式评分仍在面试结束后异步执行：

```text
Intent
  → Evidence
  → 五维 Evaluation
  → Critique / Golden Answer
```

每轮评分必须读取 `interview_turns` 自身保存的问题快照，而不是依赖候选题编号或可变化的内部题集。

评分维度、总分计算和报告撰写属于确定性 Workflow，不属于面试官 Agent 主循环。

---

## 8. 数据模型要求

### 8.1 Interview Session

Session 除现有字段外，需要具备或能够从蓝图稳定读取：

```text
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
```

同一 Session 必须固定简历版本、Prompt 版本和模型版本。

### 8.2 Interview Turn

每个真实 Turn 必须保存不可变快照：

```text
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
source_question_id（仅兜底题或锚点题可选）
```

动态生成的主问题和追问都允许 `source_question_id` 为空，但不能缺失评分所需的快照。

### 8.3 决策幂等与恢复

- 回答保存后必须有持久化的 `deciding` 状态或唯一决策记录。
- 同一个已回答 Turn 只能成功生成一次下一问。
- Worker/API 崩溃后可以根据状态恢复同一决策，不能覆盖回答或重复插题。
- 下一问写入与 Session phase 更新应在同一事务中完成。

---

## 9. Prompt 治理

```text
backend/prompts/
  resume.extract/v1/
  blueprint.generate/v2/
  interview.materials/v1/
  interview.next_turn/v2/
  evaluation.critique/v2/
  report.compose/v1/
```

- system、task、evidence、user input 必须明确分隔。
- 简历、公司资料和用户回答始终作为不可信数据注入。
- 每个输出使用 JSON Schema 和 Go 领域校验。
- 结构化输出失败最多修复一次。
- Prompt 以 `prompt_key + version` 审计，不在原版本目录直接覆盖语义。
- 下一问 Prompt 必须要求 `deepen` 和 `switch_capability` 都输出具体问题。
- Prompt 不得写死全局题数；只读取服务端传入的蓝图边界。

---

## 10. 审计、成本与模型固定

### 10.1 调用审计

`model_invocations` 至少记录：

- provider / model；
- prompt key / version；
- session / task / resource；
- 输入输出哈希；
- token 和预估费用；
- 延迟与错误码；
- trace ID / request ID；
- 对应 Turn 或 decision ID。

模型正文不直接写审计表，避免泄露简历和回答。

### 10.2 成本约束

- 每轮 Agent 采用“一次逻辑运行”，避免无工具时重复模型调用。
- 工具调用最多 2 次。
- 输入只带最近 2～3 轮和结构化进度。
- 最终逐轮评分可以受控并行，但必须限制并发。
- 每日调用额度必须按标准面试完整成本重新计算，不能让一场正常面试在中途触发硬限制。
- 达到软额度时可以切换 Session 已允许的小模型策略，但同一 Session 不应无提示地频繁切换模型。

### 10.3 同会话模型固定

创建面试时确定：

```text
interviewer_model
prompt_version
policy_version
```

后续每轮从 Session 读取并验证，不能仅记录最后一次返回的模型名称。

---

## 11. 前端实时体验

“实时提问”的核心定义是：**下一道问题在用户提交当前回答之后，基于该回答生成。** 它不等同于必须逐字流式输出。

推荐交互：

```text
用户提交回答
  → 回答立即保存
  → 页面显示“AI 面试官正在分析你的回答”
  → Session phase = deciding
  → 下一问生成完成
  → 页面收到 turn.ready 并展示新问题
```

实现可以分两步：

1. MVP 先使用异步决策任务 + 短轮询，保证崩溃恢复和请求不超时。
2. 随后升级 SSE 推送 `turn.ready`；未来做语音面试时再引入流式 ASR、TTS 和 WebSocket。

前端还必须展示：

- 当前是真实 AI 模式还是规则演示模式；
- 模型正在思考、重试或已降级；
- 降级题不能伪装成根据当前回答生成的 AI 问题。

---

## 12. RAG 与扩展能力

### 12.1 MVP 不需要通用 RAG

单份简历全文和结构化事实规模很小，可以直接放入准备阶段 Prompt。实时面试官只需通过只读工具查询当前简历版本的事实。

### 12.2 未来口子

公共面经达到较大规模后，再引入：

- Embedding；
- pgvector Retriever；
- 公司、岗位、主题过滤；
- 私有简历事实与公共面经严格隔离。

RAG、MCP 和更多工具不是修复当前主循环的前置条件。

---

## 13. 推荐目录落点

```text
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
```

- `platform/ai` 负责怎么调用和编排模型。
- `aiworkflow` 负责强类型智能节点。
- API 负责业务事务和状态转换。
- Worker 负责可恢复的异步准备、下一轮生成和报告。

---

## 14. 改造顺序

```text
1. 修改 Blueprint 和 NextTurnDecision 契约，移除固定五题
2. 增加 Session policy/phase/capability_progress 与 Turn 快照字段
3. 把内部题集改成 Rubric、锚点和兜底素材
4. 重写实时面试官：所有非结束动作都生成具体下一问
5. 将回答后推进改成可恢复的 deciding 状态机
6. 修正工具范围，绑定当前 resume_version_id
7. 报告改为直接读取 Turn 快照
8. 前端增加思考、重试、AI/规则模式和 turn.ready 体验
9. 接入真实模型完成端到端验收
10. 再考虑 Eino Graph/Workflow 细化和 SSE/语音扩展
```

不要采用以下“表面修复”：

- 只把 5 改成 10 或 15；
- 只开启 `IM_AI_ENABLED`；
- 只增加 WebSocket 或流式动画；
- 继续让 `next_capability` 从固定候选题中取下一题。

这些做法不会把题单状态机变成实时面试 Agent。

---

## 15. 验收标准

### 15.1 主循环

- [ ] AI 模式下，每个已回答 Turn 后都有对应模型调用审计。
- [ ] 下一问内容能够引用或针对当前回答，而不是固定题单顺序。
- [ ] `deepen` 和 `switch_capability` 都会生成具体下一问。
- [ ] 一场标准面试可以自然超过 5 轮。
- [ ] 面试不会超过 `max_turns` 或时间预算。
- [ ] 未达到 `min_turns` 或关键能力未覆盖时不能结束。
- [ ] 单链追问深度和整场追问预算生效。

### 15.2 状态与恢复

- [ ] 回答保存后进入持久化 `deciding` 状态。
- [ ] 重试不会覆盖答案或重复生成下一问。
- [ ] API/Worker 重启后可以恢复未完成的决策。
- [ ] 同一 Session 固定简历、模型、Prompt 和策略版本。

### 15.3 评分与记录

- [ ] 动态主问题和动态追问都保存完整评分快照。
- [ ] 报告不依赖候选题 ordinal 关联。
- [ ] 面试记录完整展示问题、回答、评分、点评和参考答案。

### 15.4 运行真实性

- [ ] AI 未启用时前端明确显示规则演示模式。
- [ ] 真实 AI 验收时 `model_invocations` 不为 0。
- [ ] 至少使用一次真实 Provider 完成“准备 → 多轮动态问答 → 报告”闭环。
- [ ] 模型失败、超时、无效输出和工具失败都有明确可观测结果。

---

## 16. 核心设计决策速查

| 问题 | 决策 |
| --- | --- |
| 多 Agent？ | 否，确定性工作流 + 单一收窄面试官 ReAct |
| 谁生成正常下一问？ | 实时面试官 Agent |
| 内部题集做什么？ | 蓝图、Rubric、锚点、期望证据和兜底素材 |
| 是否固定五题？ | 否，由蓝图轮数范围和状态机控制 |
| 谁决定结束？ | 服务端状态机；模型只能建议 |
| 默认面试长度？ | 标准 30 分钟，最少 8、目标 12、最多 16 轮 |
| 工具调用？ | 只读，每轮最多 2 次，绑定 Session 数据边界 |
| 上下文？ | 蓝图进度 + 当前回答 + 最近 2～3 轮 |
| 长期记忆？ | 不做；使用 DB 状态和可选技能画像 |
| RAG？ | MVP 不需要；公共面经上量后再接 |
| 实时是否等于逐字流式？ | 否，核心是提交回答后动态生成下一问 |
| AI 关闭时怎么办？ | 明确显示规则演示模式，不伪装成 AI |
