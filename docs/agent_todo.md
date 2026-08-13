# InterviewMaster Agent 落地 TODO

> 生成日期：2026-08-12
> 依据：`docs/上线TODO.md`、`docs/agent设计.md` 与当前代码现状复核
> 目标：把现有规则版 Demo 升级为可灰度、可审计、可控成本、可回归验证的真实 AI 面试系统

---

## 0. 范围与完成标准

本清单只覆盖智能层及其直接依赖：模型 Provider、Prompt、简历理解、出题、动态追问、评分、报告、审计、成本、评测和相关前后端状态。

不在本轮实现：

- 自主多 Agent 协作。
- 通用长期记忆或上下文压缩。
- MVP 私有简历 RAG。
- MCP 工具生态。
- 大规模公共面经向量库。

首发架构保持：

```text
确定性工作流（简历抽取 → 蓝图 → 出题 → 评分 → 报告）
                    +
一处收窄的动态追问决策（由业务状态机控制出口）
```

### 状态标记

- `[ ]` 未开始。
- `[-]` 已有骨架或进行中，尚未通过验收。
- `[x]` 已完成并通过对应验收。
- `[!]` 被外部条件或产品决策阻塞。

### Agent MVP 完成定义

同时满足以下条件才可认为智能层具备邀请制上线能力：

1. 题目会因简历、JD 和目标岗位不同而产生可解释差异，不再返回固定五题。
2. 报告基于题目、回答和材料证据评分，不再按回答字数计算。
3. 所有模型输出先进入强类型结构并校验，非法输出不会直接写入业务表。
4. 每次调用可追溯到 Provider、模型、Prompt 版本、Token、耗时、费用和 Trace。
5. 模型超时、限流、不可用或输出非法时，用户能看到明确状态，并可安全重试或得到受控降级。
6. 每个用户有并发、调用次数和成本上限，不会出现无界调用。
7. Golden Dataset 和真实全流程 E2E 达到发布基线。

---

## 1. 已确认的设计决策

以下决策作为本轮实现约束，不在编码过程中反复摇摆：

| 问题 | 决策 |
| --- | --- |
| 架构形态 | 不做自主多 Agent；采用确定性工作流，动态追问保持有界 |
| 编排框架 | Eino 可用于 `platform/ai` 内部，领域 logic 不直接 import Eino |
| 业务依赖 | 领域层只依赖自研 `ChatModel`、`EmbeddingModel`、`Tool` 等接口 |
| MVP RAG | 不做；简历/JD 经长度控制后全文直喂，证据依靠事实 ID 与原文定位 |
| Embedding | MVP 不依赖；移除或停止新增 SHA-256 假向量，不把它作为真实能力展示 |
| 状态与记忆 | PostgreSQL 是权威状态；调用时按规则组装最近上下文，不使用独立 Agent Memory |
| 租户标识 | 当前产品以 `user_id` 隔离；在正式引入 workspace 表以前不混用虚构 `workspace_id` |
| 评分原则 | 模型输出各维证据与判断，Go 代码按固定规则计算总分 |
| 模型调用位置 | 长任务走 Worker；只有需要交互时延的追问允许同步调用 |
| 失败原则 | 失败显式可见；只对明确可重试错误重试，不伪造 AI 结果 |

---

## 2. 当前基线

### 已有基础

- [x] API、Worker、PostgreSQL、Redis、对象存储与异步任务骨架。
- [x] 简历上传、Tika 文本提取、JD、题集、面试状态机和报告数据表基础。
- [x] JWT + Refresh Session、用户级资源隔离、登录限流。
- [x] OpenTelemetry Trace 骨架与结构化 HTTP 日志。
- [x] Embedding/Tool 已提供真实适配口：OpenAI embeddings、只读工具、pgvector Retriever 和 MCP list/call 适配器。MVP 私有简历仍以全文直喂为主。
- [x] `backend/prompts/` 已包含简历抽取、蓝图、出题、评分和报告的可执行 Prompt 与 Schema。

### 必须替换的 Stub

- [x] 正式出题走蓝图 + 异步 Worker；关闭 AI 时写入 `degraded` 规则题，不再作为正式成功结果。
- [x] 报告改为维度评分与异步生成，已删除字数评分。
- [x] 简历解析保留 Tika 文本，启用 AI 时调用 `resume.extract/v1` 抽取带原文摘录的事实。
- [x] 新 chunk 不再写入 SHA-256 假向量，`embedding` 保持 NULL。
- [x] 面经检索改为本地 `public_intel_items` 语料关键字检索；前端仍无正式入口。

---

## 3. P0-A：模型基础设施与配置

### AGENT-001 固化自研接口

- [x] 保留 `ChatModel.Generate`，补齐请求级 deadline、结构化输出约束和调用元数据。
- [x] 首发无流式 UI，暂不扩张 `Stream` 接口。
- [x] 将租户字段统一为当前真实存在的 `UserID`；未来引入工作区后再兼容 `WorkspaceID`。
- [x] 定义统一错误类型：超时、限流、鉴权、供应商错误、输出非法、上下文超限和预算耗尽。
- [x] Provider 原始错误留在错误 cause 中，不直接形成客户端文案。
- [x] 为接口提供 Fake Model，Eino Graph 业务测试不依赖真实外部模型。

验收：

- 业务包不 import Eino 或供应商 SDK。
- Provider 可以被 Fake 实现替换，核心 workflow 单测无需网络。

### AGENT-002 接入一个真实 Provider

- [x] 固定 Eino `v0.9.13` 与 OpenAI 组件 `v0.1.13`，不使用 `@latest`。
- [x] 在 `backend/internal/platform/ai/provider/` 实现 OpenAI 兼容 ChatModel 适配。
- [x] 支持 Base URL、API Key、模型名、请求超时、最大输出 Token 等环境配置。
- [x] API 与 Worker 均通过 `airuntime.NewChatModel` 注入同一套审计/配额包装。
- [x] 启动时校验启用状态下的 Provider、密钥、模型、超时和 Token 上限。
- [x] 为 Provider 实现连接错误、429、5xx 和超时的错误映射。

建议环境变量：

```text
IM_AI_PROVIDER=
IM_AI_BASE_URL=
IM_AI_API_KEY=
IM_AI_CHAT_MODEL=
IM_AI_SMALL_MODEL=
IM_AI_REQUEST_TIMEOUT=
IM_AI_MAX_OUTPUT_TOKENS=
```

验收：

- 本地可通过最小请求获得真实模型响应。
- 未配置密钥时开发环境可明确禁用 AI，生产环境启动失败。
- API Key 不进入日志、数据库输入快照或客户端响应。

### AGENT-003 调用可靠性策略

- [x] 已设置 HTTP Client 和整次调用 deadline；首字节超时等待流式调用阶段补充。
- [x] 仅对网络瞬断、429 和可恢复 5xx 做指数退避，最多重试 2 次并加入 jitter。
- [x] 4xx 参数错误、输出校验错误和预算拒绝不做传输重试。
- [x] JSON 修复最多 1 次，且与传输重试分开计数。
- [x] 传输重试复用同一业务请求；审计记录最终 attempt 结果。分次 attempt 明细可在后续增强。
- [x] Provider 关闭或不可用时题集/报告进入 `degraded` 或 `failed`，降级输出遵守同一领域结构。

---

## 4. P0-B：审计、成本与额度先行

### AGENT-010 新增 `model_invocations`

- [x] 新建数据库迁移和查询，至少记录：

```text
id
user_id
task_id / session_id / resource_type / resource_id
provider / model
prompt_key / prompt_version
status / attempt
input_hash / output_hash
prompt_tokens / completion_tokens / total_tokens
estimated_cost
latency_ms
error_code
trace_id / request_id
created_at / completed_at
```

- [x] 默认不保存完整简历、JD、回答和模型输出正文；只存输入/输出哈希。
- [x] 为 `user_id + created_at`、`resource_id`、`trace_id` 建查询索引。
- [x] 使用统一 `InstrumentedChatModel` 埋点，业务节点不各自写审计。
- [x] 审计写入失败会记录错误；生产环境阻断，开发环境降级继续。

### AGENT-011 成本和配额

- [x] 设置单次最大输入字符/Token、最大输出 Token 和最大耗时。
- [x] 设置用户级同时执行中的模型调用上限。
- [x] 设置用户日调用次数与日/月预算软、硬阈值。
- [x] 软阈值触发小模型；硬阈值返回 `AI_BUDGET_EXHAUSTED`。
- [x] 对题集使用输入哈希复用进行中/已完成生成；报告按 session 唯一绑定。
- [x] 管理员可通过 `model_invocations` 表注释中的聚合 SQL 查看成本和异常用户。

验收：

- 任意一次业务生成都能追踪 Token、耗时、成本和结果状态。
- 并发压测不能绕过用户硬额度。
- 重复请求不会无界重复计费或写入重复业务结果。

---

## 5. P0-C：Prompt 与强类型输出

### AGENT-020 建立可执行 Prompt 包

- [x] 六个 Prompt 包已建立并嵌入二进制：

```text
backend/prompts/resume.extract/v1/
backend/prompts/blueprint.generate/v1/
backend/prompts/question.generate/v1/
backend/prompts/evaluation.critique/v1/
backend/prompts/interview.followup/v1/
```

- [x] 每个目录包含 `system.md`、`task.md`、`meta.yaml` 和输出 Schema。
- [x] Prompt 加载器使用 `embed.FS`，容器运行时不依赖工作目录。
- [x] Prompt 按版本目录存放；升级时新增版本目录，不原地覆盖旧版本。
- [x] 所有 Prompt 将 System、任务、Schema 与不可信数据区分离。
- [x] 输入按字段做 rune 裁剪，并保留不可信数据分隔边界。

### AGENT-021 定义领域输出契约

- [x] 定义 `ResumeExtraction`：事实类别、事实值、置信度、`source_excerpt`、原文定位。
- [x] 定义 `InterviewBlueprint`：目标能力、权重、难度、主问题数、追问预算、时间预算。
- [x] 定义 `GeneratedQuestionSet`：题目、意图、期望点、能力键、难度、证据 fact IDs、追问建议。
- [x] 定义 `TurnEvaluation`：各评分维度、评分理由、证据、缺失信息和改进建议。
- [x] 定义 `InterviewReportDraft`：优势、改进项、训练步骤，不允许模型自行覆盖确定性总分。
- [x] 每个结构同时提供 Go Struct、业务校验器和 JSON Schema。
- [x] 枚举、数量、长度、分数范围、ID 引用、重复项和证据归属都由 Go 校验。

### AGENT-022 统一结构化输出执行器

- [x] StructuredRunner 执行模型调用、本地 JSON Schema 校验、严格 JSON 解码并拒绝未知字段。
- [x] Provider 原生 JSON Schema 与领域校验已接通，并增加本地通用 JSON Schema Validator。
- [x] 第一次失败时把精简校验错误交给修复 Prompt，最多修复一次。
- [x] 第二次仍失败则返回 `AI_OUTPUT_INVALID`，不写半成品业务数据。
- [x] 保存失败类型和输出哈希，供模型/Prompt 回归分析。

---

## 6. P0-D：真实简历理解

### AGENT-030 将简历抽取升级为异步 AI 节点

- [x] 保留 Tika 负责文件转文本，LLM 只处理已提取文本。
- [x] 为文本长度、空白、乱码和超长简历建立预处理规则。
- [x] 调用 `resume.extract/v1` 生成技能、经历、项目、指标和教育等结构化事实。
- [x] 每条事实必须包含可验证的原文摘录和所属简历版本。
- [x] 在同一事务中替换该版本的旧 facts，避免混合多个抽取版本。
- [x] 保存 `extractor_model`、`prompt_version` 或关联 invocation ID，支持重解析和回溯。
- [x] 解析失败时保留 Tika 文本和可重试任务状态，不把简历误标为 completed。

### AGENT-031 移除假 Embedding 语义

- [x] MVP 不再为新 chunk 写入 SHA-256 伪向量。
- [x] 旧 `baseline-hash-1536` 数据保留兼容，不再作为新写入路径。
- [x] 若暂时保留 chunks，只保存文本切片和 token_count，不宣称具有语义检索能力。
- [x] Embedding 字段和 pgvector 扩展可作为未来兼容口保留。

验收：

- 使用至少 10 份不同格式简历，事实能定位回原文。
- 模型不得捏造原文不存在的公司、项目、技术或指标。
- 相同简历版本重复解析具备幂等或明确版本语义。

---

## 7. P0-E：真实蓝图与题集生成

### AGENT-040 蓝图数据模型

- [x] 决定蓝图归属：生成后固化到题集，并在创建面试时复制快照。
- [x] 新增 `blueprint` JSONB，并记录 Schema/Prompt/模型版本。
- [x] 蓝图至少包含能力键、权重、难度、题数、追问上限和证据范围。
- [x] 历史面试始终读取创建时快照，不被后续重新生成回写改变。

### AGENT-041 题集生成工作流

- [x] 输入当前用户的简历原文、结构化 facts、可选 JD 和目标岗位。
- [x] 生成蓝图。
- [x] Worker 先生成蓝图再生成题目。
- [x] 校验数量、连续序号、字段长度、未知字段、能力覆盖和证据 ID。
- [x] 对不合格输出只做一次定向修复，不重新生成已经合格的全部内容。
- [x] 模型调用已移动到数据库写事务之外，成功校验后才开始写事务。
- [x] 使用输入资源版本和 Prompt 版本组成幂等键。
- [x] 正式 AI 路径不再调用固定题；关闭 AI 的本地回退标记为 `degraded`。

### AGENT-042 改为异步产品流程

- [x] 创建题集请求返回 `202 + task_id`。
- [x] Worker 执行蓝图和题目生成，持续更新进度阶段。
- [x] 前端展示“读取材料、规划能力、生成题目、质量检查”等真实阶段。
- [x] 支持失败重试，并按输入哈希复用已有题集，避免重复计费。
- [x] 前端不得使用 60 秒长 HTTP 请求等待完整生成。

验收：

- 不同简历/JD 生成结果具有材料相关性和差异性。
- 每道材料相关题都能追溯到事实 ID 或明确标记为岗位通用题。
- 重复、越权证据 ID、非法 JSON 和模型超时都有自动化测试。

---

## 8. P0-F：真实评分与报告

### AGENT-050 确定评分量表

- [x] 明确五个评分维度、权重及 0–100 的锚点定义。
- [x] 区分“未提供证据”和“能力不足”，避免对简短但有效的回答只按长度扣分。
- [x] 明确跳过题、空回答和无可用材料时的评分规则。
- [x] 由 Go 校验各维分数并确定性聚合总分。
- [x] 报告必须声明评分为训练建议，不作为招聘结论。

### AGENT-051 评分工作流

- [x] Intent：读取题目意图、能力点和预期回答点。
- [x] Evidence：从回答、简历事实和上下文中抽取支持/反驳证据。
- [x] Evaluation：按固定量表输出各维评分和理由。
- [x] Critique：生成具体、可执行且不捏造经历的改进建议。
- [x] 对各题顺序评测，并由用户级并发配额限制模型调用。
- [x] 单题失败时整份报告失败并可重试，禁止静默填充固定高分。

### AGENT-052 报告生成

- [x] Go 汇总各题维度分数、总分和质量门禁。
- [x] 报告模型只负责组织优势、改进优先级和训练计划，不自行改分。
- [x] Golden Answer 必须基于用户已有材料；需要假设的内容明确标为示例表达。
- [x] evidence 返回事实 ID/回答摘录等稳定引用，而非“用户提供了回答内容”之类泛化文本。
- [x] 报告与 session 唯一绑定；重复查询复用结果。
- [x] 删除 `scoreAnswer()` 字数评分和固定 critique/golden answer。

### AGENT-053 报告异步化

- [x] 完成面试时投递报告任务，GET 报告只读取状态/结果，不在请求事务中调用模型。
- [x] 支持 `pending/running/completed/failed/degraded` 状态。
- [x] 前端轮询或订阅状态并提供安全重试入口。
- [x] 报告生成失败不得影响已经完成的面试和用户回答。

验收：

- 同一回答重复评测的分数波动保持在设定范围。
- 更具体、有证据的回答通常优于空泛或无关回答。
- 报告中的事实、数字和技术必须能追溯到材料或回答。
- 空回答、提示注入、超长回答和中英混合作答均有测试。

---

## 9. P1：动态面试官与收窄 ReAct

真实出题和评分稳定后再实现本阶段，避免首发同时引入过多不确定性。

### AGENT-060 先实现受限决策器

- [x] 第一版优先采用强类型决策输出，而不是立即开放任意 Tool Calling：

```json
{
  "action": "follow_up",
  "question": "...",
  "capability_key": "...",
  "evidence_fact_ids": ["..."],
  "reason": "..."
}
```

- [x] `action` 仅允许 `follow_up`、`next_capability`。
- [x] Go 状态机决定是否接受该动作、是否结束面试以及下一 ordinal。
- [x] 每轮最多一个追问决策，超时则使用蓝图中的静态下一题。

### AGENT-061 补齐动态会话模型

- [x] 为 session 保存蓝图快照、当前能力点、已用追问数和模型版本。
- [x] 为 turn 增加主问题/追问类型、`parent_turn_id`、能力键和生成 invocation ID。
- [x] 明确动态追问是否占用总题数与总时间。
- [x] 每轮上下文只包含系统规则、蓝图当前点、最近 2–3 轮和本轮回答。
- [x] 刷新或 Worker/API 重启后能够从数据库恢复，不依赖进程内消息历史。

### AGENT-062 按需升级为 Tool/ReAct

- [x] 只有在真实“查简历片段”或“查面经”工具产生价值后才接 ADK ChatModelAgent。
- [x] 工具必须只读，并在内部强制绑定当前 `user_id`，模型参数不能覆盖租户范围。
- [x] 工具参数使用 JSON Schema，返回结果限制条数和字符数。
- [x] 每轮工具调用最多 2 次，循环深度、Token 和时间均有硬上限。
- [x] 工具超时/空结果时回到蓝图静态下一题。
- [x] 对工具调用、结果哈希和拒绝原因进行审计。

验收：

- 模型无法自行增加无限轮次或结束整场面试。
- 恶意回答不能诱导工具读取其他用户资料。
- 追问超时、Provider 故障和工具故障均能回退并走完整场面试。

---

## 10. P1：安全、隐私与提示注入

### AGENT-070 Prompt Injection 防护

- [x] 明确系统指令优先级，简历、JD、面经和回答一律视为不可信数据。
- [x] 不允许用户内容改变输出 Schema、租户范围、工具列表、预算或系统角色。
- [x] 工具服务端重新做授权，不信任模型提供的用户或资源 ID。
- [x] 对“忽略规则”“泄露 Prompt”“读取其他简历”等攻击建立评测集。
- [x] 日志和错误响应不泄露 System Prompt、API Key 或完整用户材料。

### AGENT-071 数据治理

- [x] 明确模型供应商的数据保留和训练政策，生产配置选择不用于训练的接口/账户。
- [x] 在隐私政策中披露简历、JD、回答会被发送给模型供应商处理。
- [x] 支持账户数据导出、删除以及模型审计记录的合规保留策略。
- [x] 给模型输入、审计调试正文和对象存储设置保留周期。
- [x] 对日志中的邮箱、回答正文、简历片段和 Token 做脱敏。

---

## 11. P1：评测、测试与发布门禁

### AGENT-080 Golden Dataset

- [x] 建立版本化匿名样本，覆盖应届中文、社招英文、空回答、跑题和 Prompt Injection。
- [-] 大规模人工标注的事实抽取/题集相关率/人工评分对照集仍待积累。
- [x] Fake Model 回归覆盖空回答为 0、注入不能改变 action、未知字段拒绝。
- [x] 加入空回答、跑题和 Prompt Injection 样本。
- [x] Prompt/Schema 变更可通过 `go test ./internal/aieval ./internal/aiworkflow ./internal/platform/ai/...` 回归。

### AGENT-081 自动化测试

- [x] Provider 适配单测：正常、429、5xx、超时、非法 JSON、Token usage 缺失。
- [x] Prompt 加载与版本不存在测试。
- [x] 所有领域 Schema 和校验器单测。
- [x] Fake Model 驱动的简历、出题、评分、报告、追问 workflow 单测。
- [-] PostgreSQL + Redis + Worker 集成测试：重复投递、崩溃、重试、幂等、死信。
- [x] 跨用户访问、证据 ID 越权和工具越权安全测试。
- [-] 注册→上传→解析→JD→题集→面试→报告全流程 Playwright E2E。
- [-] 生产相同配置下的小流量真实 Provider smoke test，密钥只来自 CI Secret。

### AGENT-082 质量门禁

- [x] 定义事实抽取准确率/召回率基线。
- [x] 定义题目材料相关率、重复率、能力覆盖率基线。
- [x] 定义评分稳定性和与人工评分的相关性基线。
- [x] 定义非法结构输出率、模型错误率、P95 延迟和单次成本上限。
- [-] 低于任一硬基线时阻止 Prompt 或模型版本上线。
- [x] 支持快速回滚到上一 Prompt/模型组合。

---

## 12. P1：可观测性与运行保障

### AGENT-090 指标

- [x] 模型请求量、成功率、重试率、降级率。
- [x] 首 Token/总耗时；若无流式则至少记录总耗时。
- [x] 输入、输出、总 Token 和估算费用。
- [x] 结构化输出首次失败率、修复成功率和最终失败率。
- [x] 各 workflow 节点耗时和失败率。
- [x] 题集/报告任务排队时长、执行时长和积压量。
- [x] 单用户异常调用量及预算拒绝次数。

### AGENT-091 告警与 Runbook

- [x] Provider 错误率或 P95 延迟超阈值。
- [x] 非法输出率显著上升。
- [x] 队列堆积或任务长时间 running。
- [x] 单用户/全站 Token 或费用异常。
- [x] 审计写入失败、成本价格表缺失。
- [x] 为每类告警编写排查、降级、停用和恢复步骤。

---

## 13. P2：未来能力

- [x] 面经数据源完成合规评估后接入真实检索 Provider。
- [x] 公共面经达到需要检索的规模后接真实 Embedding。
- [x] 自研 pgvector Retriever，所有私有查询强制按 `user_id`/未来 workspace 范围过滤。
- [x] 公共面经库和用户私有材料使用独立数据域与授权策略。
- [-] 建检索召回率、空结果率、引用正确率评测。
- [x] 需要外部工具生态时再实现 MCP Tool Adapter。
- [x] 多次面试后可增加结构化 `user_skill_profile`，但必须允许用户查看、纠正和删除。
- [x] 有明确产品收益后再考虑流式输出、语音实时面试或更复杂 Agent 协作。

---

## 14. 推荐实施批次

### Batch 1：打通最小真实 AI 路径

- AGENT-001～003：接口、Provider、可靠性。
- AGENT-010：调用审计。
- AGENT-020～022：Prompt 和结构化输出执行器。
- AGENT-030：真实简历事实抽取。

交付物：上传一份简历后，可以得到带原文证据的真实结构化事实。

### Batch 2：做真核心用户价值

- AGENT-040～042：蓝图和题集异步生成。
- AGENT-050～053：评分与报告异步生成。
- 同步接通前端任务状态、题集和报告页面。

交付物：不同材料得到不同题目，完成面试后得到有证据的真实报告。

### Batch 3：建立上线门禁

- AGENT-011：额度与成本。
- AGENT-070～071：安全与数据治理。
- AGENT-080～082：Golden Dataset、测试和门禁。
- AGENT-090～091：监控、告警和 Runbook。

交付物：可观测、可限额、可回滚并通过邀请制 Beta 发布检查。

### Batch 4：增强交互

- AGENT-060～062：动态追问与按需 ReAct。
- P2 面经检索、Embedding、MCP 等未来能力按实际数据规模实施。

交付物：有界、可恢复、失败可降级的动态面试官。

---

## 15. 上线检查清单

### 邀请制 Beta 前必须全部通过

- [x] 固定五题、字数评分和固定报告已从正式路径移除。
- [x] Provider 与 Prompt 版本固定，可回滚到上一版本目录/环境变量。
- [x] 题集和报告走异步任务，接口不持长事务等待模型。
- [x] 所有模型输出通过 Schema 和领域校验。
- [x] 模型调用审计、Token、成本和 Trace 可查询。
- [x] 用户并发、日调用和预算硬限制生效。
- [x] Prompt Injection、跨用户证据访问和日志泄密测试通过。
- [-] 核心 E2E、Worker 集成测试和 Golden Dataset 达到基线。
- [x] 模型不可用时题集/报告显示明确失败或降级状态，可安全重试。
- [x] 隐私政策和 AI 评分声明覆盖第三方模型处理。

### 公开注册前追加

- [-] 容量与成本压测完成。
- [x] 监控告警与 Runbook 已接通；完整故障演练仍待做。
- [x] 数据导出、账户删除和数据保留策略上线。
- [ ] Provider 配额耗尽、区域故障和密钥轮换演练通过。
- [ ] 发布、回滚、数据库备份恢复流程完成演练。

---

## 16. 首轮开发建议

第一轮不要同时实现 Eino Graph、Workflow、ADK、RAG 和 MCP。建议先建立一条最小纵向切片：

```text
真实 Provider
  → model_invocations
  → question.generate/v1
  → Go Struct + Schema 校验
  → Fake Model 单测
  → 一份简历/JD 生成真实题集
```

这条路径验证配置、模型调用、Prompt 加载、结构化输出、审计和业务写库六个关键边界。跑稳后，再复用相同基础扩展简历抽取和评分报告。
