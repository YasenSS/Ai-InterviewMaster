// Package ai 是智能层基建包（AI Orchestrator 的落点）。
//
// 分层职责：
//   - 本包（platform/ai）负责「怎么调模型」：Provider、审计、编排实现、护栏。
//   - 领域层（apps/*/internal/logic/workspace）负责「用模型干什么」：
//     出题、面试官追问、评分、报告等业务编排在 logic 方法内，
//     通过 svcCtx 注入的 ChatModel 调用本包，不感知 Eino/供应商。
//
// 文件规划（逐步落地，见 docs/agent设计.md §8/§9）：
//   - model.go            ChatModel / EmbeddingModel / Tool / Message 等自研接口（已建）
//   - provider/openai.go  OpenAI 兼容 Provider（已建，基于 Eino）
//   - structured.go       生成→严格 JSON 解码→领域校验 Graph（已建）
//   - audit.go            model_invocations 审计写入（Eino Callback 钩子）
//   - graph_question.go   出题 Graph 编排（生成→校验→去重）
//   - graph_eval.go       评分 Workflow 编排（Intent→Evidence→Evaluation→Critique）
//   - interviewer.go      面试官收窄 ReAct 循环（ADK ChatModelAgent + 只读检索工具）
//   - retriever_pgvector.go （未来）自研 pgvector Retriever，保留 workspace_id 隔离
package ai
