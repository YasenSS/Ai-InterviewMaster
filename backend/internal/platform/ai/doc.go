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
//   - structured.go       生成→JSON Schema→严格解码→领域校验，失败修复一次
//   - retry.go / quota.go / audit.go / instrumented.go  重试、额度、审计包装
//   - contract/           简历/蓝图/内部候选题/评分/报告/追问决策的强类型输出
//   - react.go            面试官收窄 ReAct 循环（最多 2 次只读工具）
//   - tools/              简历事实与本地面经检索，强制绑定 user_id
//   - retriever/          自研 pgvector Retriever，私有查询强制 user_id
//   - mcp/                Tool 的 list/call 适配口，不接外部 MCP server
package ai
