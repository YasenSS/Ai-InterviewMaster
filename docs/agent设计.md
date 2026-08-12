# InterviewMaster Agent 设计

> 生成日期：2026-08-12
> 定位：智能层（Agent / LLM / 编排）的落地设计
> 依据：`技术方案_v1.md` §4/§6 + 当前代码现状 + 设计讨论结论

---

## 0. 一句话结论

**不做「自主多 Agent」，做「确定性工作流（Graph）+ 一处收窄的 ReAct 循环（面试官追问）」。**

- 主流程（简历抽取 → 出题 → 评分 → 报告）是已知步骤，用工作流编排，不用 Agent 自主规划
- 全系统唯一接近「自主 Agent」的是**面试官每轮追问**，且范围收窄、由蓝图状态机控制出口
- MVP **不需要 RAG**、不需要跨会话记忆、不需要上下文压缩

---

## 1. 现状与框架选型

### 1.1 现状

- `backend/go.mod` **没有任何 LLM / Agent 框架依赖**（无 LangChainGo、无 Eino、无 LLM SDK）
- 出题 / 评分 / 报告 / 面经 全是 **stub**（固定模板、字数评分、哈希假向量）
- 真外部智能只有 **Tika**（文本抽取）和 **Whisper**（ASR）

### 1.2 框架选型：Eino（藏在自研接口后面）

| 候选 | 判断 |
| --- | --- |
| LangChainGo | ❌ Chain 抽象不可替换，方案 §6.1 已排除作为核心架构 |
| 自研纯接口 | ⚠️ 可控但编排要自己写，工作流复杂后成本高 |
| **字节 Eino** | ✅ 选用。Graph/Workflow 图编排贴合「确定性流水线」，ADK 支持面试官追问；Go 原生、活跃 |

**工程红线**：领域 logic **不直接 import Eino**。Eino 只作为 `platform/ai` 内部的实现，领域层只依赖自研接口，未来可替换。

```bash
go get github.com/cloudwego/eino@latest
go get github.com/cloudwego/eino-ext/components/model/openai@latest   # 或 ark / claude
go get github.com/cloudwego/eino-ext/components/embedding/openai@latest
```

---

## 2. 总体架构：多节点工作流，而非多 Agent

### 2.1 把「多 agent」翻译成「专业化链节点」

每个节点 = **一次受控 LLM 调用 + 强类型输出 + JSON Schema 校验**，节点之间用 Go/Eino Graph 编排：

| 节点 | 输入 | 输出 | 模型强度 | 编排 |
| --- | --- | --- | --- | --- |
| 简历事实抽取 | Tika 全文 | 结构化事实 JSON（技能/项目/指标 + source_span） | 小模型 | Chain |
| 面试蓝图生成 | 简历事实 + JD 能力 | InterviewBlueprint（轮数/能力分布/难度曲线/时间预算） | 中 | Chain |
| 出题器 | 蓝图 + 简历/JD 全文 | 题目/考察点/回答要点/追问树 | 强 | Graph（含校验/去重分支） |
| **面试官（每轮）** | 蓝图 + 当前能力点 + 近几轮问答 + 上轮回答 | 追问 or 下一题 | 中 | **收窄 ReAct 循环**（见 §4） |
| 评估器 | 题目 + 回答 + 证据 | 五维评分 + critique | 强 | Workflow（Intent→Evidence→Evaluation→Critique） |
| 报告撰写 | 全部评分 + 证据 | 总评 + 训练计划 | 中 | Chain |

### 2.2 Eino 编排选型

| Eino API | 用途 |
| --- | --- |
| **Chain**（线性） | 简历抽取、报告撰写 |
| **Graph**（分支/并行） | 出题（生成→校验→去重→覆盖率检查） |
| **Workflow**（字段级映射） | 评分多阶段 |
| **ADK ChatModelAgent** | 仅面试官追问（ReAct，挂只读检索工具） |

---

## 3. RAG：MVP 不需要，留口子

### 3.1 为什么 MVP 不做 RAG

单用户私有数据**极小**（1~几份简历，全文几千字），**全文直喂 prompt 即可**，不触发 RAG 的存在前提（语料大到塞不进上下文）。

| RAG 典型场景 | 本项目 | 需要吗 |
| --- | --- | --- |
| 海量语料检索 | 单用户 1 份简历 | ❌ |
| 上下文装不下 | 简历全文可装 | ❌ |
| 跨用户公共知识库 | MVP 不共享 | ❌ |

**MVP 智能层只需：ChatModel + Prompt 治理 + 结构化输出**，连 Eino 的 RAG 组件都暂用不上。

### 3.2 证据引用不靠向量

出题/评分的「证据定位」= 抽取事实时让模型带原文片段 + 存的 `source_span`，**是定位不是检索**。

### 3.3 未来口子

Beta 面经库上量（几万条+）后再引入：Embedding 用 eino-ext 现成实现，**自研 pgvector Indexer/Retriever**（几十行 SQL，保留 `workspace_id` 过滤），私有库与公共库分离。当前 `resumeparse.go` 的 SHA-256 假向量届时再换成真 embedding。

---

## 4. 面试官主循环（参考 Claude Code，但收窄）

### 4.1 Claude Code 的循环本质

ReAct loop：模型每轮决定「输出文本 or 调工具」，不再调工具即停。**模型自主控制退出。**

### 4.2 关键差异：出口由蓝图状态机控制

Claude Code 任务开放（自己决定何时停）；你的面试**有界**（蓝图定了轮数/时间预算）。所以**模型只在单轮内决定「问什么 + 要不要先查证」，停不停由状态机说了算**。

```text
blueprint = { 总轮数, 能力分布, 难度曲线, 时间预算 }   # 面试开始前确定性生成

for turn in 1..blueprint.总轮数:
    # ── 收窄的 ReAct 循环（仅本轮）──
    loop:
        resp = LLM(系统规则 + 蓝图当前能力点 + 近几轮问答 + 上轮回答,
                   tools = [查简历片段, 查面经])          # 只读工具
        if resp.tool_call:
            msgs.append(execute(tool))                   # 查证结果回灌
            continue
        else:
            break                                        # 得到本轮问题
    # ── 状态机接管 ──
    保存问答 → 用户作答 → advanceInterview 下一能力点
```

### 4.3 对照取舍

| Claude Code 做法 | 面试官是否采用 |
| --- | --- |
| 模型自主决定停止 | ❌ 由蓝图轮数/时间预算决定停 |
| 工具调用循环 | ✅ 收窄：仅「查简历/查面经」只读，每轮 ≤2 次 |
| 上下文累积 | ✅ 但裁剪：只带「当前能力点 + 近 2~3 轮」 |
| 权限确认 | ❌ 不需要（只读检索） |
| 中断恢复 | ✅ 复用现有会话状态机 |
| 成本/轮次护栏 | ✅ 必须：每轮记 token，超预算降级为「按蓝图顺序问下一题」 |

### 4.4 兜底护栏

循环超深 / 超预算时，强制退化为「按蓝图顺序出下一题」，保证面试一定能走完。

---

## 5. 上下文与记忆管理

### 5.1 结论

- **不需要** Claude Code 式通用长期记忆 / 上下文压缩
- **必须做**会话内上下文组装——但你的 Postgres 已经把会话状态全记了

### 5.2 你要的是「状态」不是「记忆」

`interview_sessions`（蓝图、current_ordinal）+ `interview_turns`（每轮问答）就是你的记忆，比 LLM context 更可靠、可恢复。每次调用前从 DB 读并组装：

```text
面试官要提问
  → 从 DB 读：蓝图 + 当前能力点 + 最近 N 轮问答
  → 裁剪组装进 prompt（控制 token）
  → LLM 生成追问 → 写回 turns
```

### 5.3 三件具体的事

1. **窗口控制**：不全塞历史，只带「系统规则 + 蓝图当前能力点 + 近 2~3 轮 + 本轮回答」。是裁剪不是压缩
2. **持久化 = 断点恢复**：已有会话状态机，LLM 侧重读 DB 即可
3. **（Beta 可选）画像沉淀**：面试结束把薄弱点/强项抽进 `user_skill_profile` 表，下次喂给蓝图生成——结构化写库，非 LLM 记忆组件

---

## 6. Prompt 治理（重点是治理不是写法）

```text
backend/prompts/
  resume.extract/v1/    system.md, task.md, meta.yaml
  blueprint.generate/v1/
  question.generate/v1/
  interview.followup/v1/
  evaluation.critique/v1/
```

- **分层分隔**：system / task / evidence / user_input 明确隔离（防注入第一道）
- **版本化**：每个 `prompt_key + version`，调用记录进审计表
- **强类型输出**：每个 Prompt 配 JSON Schema，输出入 Go struct 校验；失败只允许一次修复重试
- **用户内容当数据**：简历/JD/回答永远注入带分隔的「数据区」，不与指令混排

---

## 7. 审计与成本治理

- **`model_invocations` 表**（先于任何真实调用建）：provider/model/prompt_key/version/输入输出哈希/token/费用/耗时/trace_id
- **Eino Callback 埋点**：用 `callbacks.Handler` 在 OnEnd 取 TokenUsage 写审计表，不侵入业务
- **成本护栏**：每次调用最大输入/输出/超时、按用户并发限制；工作区日/月预算软硬阈值
- **同会话固定模型版本**，避免中途变化体验不一致

---

## 8. 目录落点

| 关注点 | 位置 | 说明 |
| --- | --- | --- |
| 模型调用 / 编排实现 | `backend/internal/platform/ai/`（新建） | Eino 藏这里，领域不 import |
| 业务编排（Agent 节点） | 现有 `apps/api/internal/logic/workspace/*` | 出题/面试/评分的 logic 方法内调 `svcCtx.ChatModel` |
| 简历抽取 | `apps/worker/internal/tasks/resumeparse.go` | 异步 worker 节点 |
| Prompt 模板 | `backend/prompts/`（新建，根级） | 与代码分离、版本化 |
| 依赖注入 | `apps/api/internal/svc/servicecontext.go` | 加 ChatModel/EmbeddingModel 字段 |

`platform/ai/` 内部建议：

```text
platform/ai/
  model.go              # ChatModel / EmbeddingModel / Tool 自研接口
  provider_openai.go    # Eino ChatModel 实现（包在自研接口后）
  graph_question.go     # 出题 Graph
  graph_eval.go         # 评分 Workflow
  interviewer.go        # ADK ChatModelAgent（追问）
  audit.go              # model_invocations 写入（Callback）
  retriever_pgvector.go # （未来）自研 pgvector Retriever
```

---

## 9. 落地顺序

```text
1. 定义 ChatModel/EmbeddingModel/Tool 自研接口 + Eino 实现一个 Provider
2. 建 model_invocations 审计表 + Eino Callback 埋点（先于真实调用）
3. 出题节点跑通：真实 Prompt + 简历/JD 全文直喂 + JSON Schema 输出（不碰 RAG）
4. 面试官追问节点：收窄 ReAct + 只读检索工具 + 兜底护栏
5. 评分/报告多阶段节点（Workflow）
6. 全程成本/token 埋点 + Golden Dataset 评测
7.（未来）面经上量再接 pgvector Retriever；工具生态需要再接 MCP
```

---

## 10. 核心设计决策速查

| 问题 | 决策 |
| --- | --- |
| 多 Agent 架构？ | 否。确定性工作流（Graph）+ 一处收窄 ReAct（面试官） |
| 框架？ | Eino，藏在自研接口后，领域不直接依赖 |
| RAG？ | MVP 不需要；全文直喂。留 pgvector 口子给 Beta 面经 |
| 主循环参考 Claude Code？ | 仅面试官每轮追问；出口由蓝图状态机控制 |
| 上下文/记忆管理？ | 无独立记忆系统；DB 状态机 + 调用前裁剪组装 prompt |
| 上下文压缩？ | 不需要；面试有界塞得下 |
| MCP？ | 留 Tool 接口口子，MVP 不接 |
