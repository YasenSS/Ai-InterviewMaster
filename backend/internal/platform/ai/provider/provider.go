// Package provider 存放各模型供应商的 ChatModel/EmbeddingModel 实现。
//
// 每个供应商一个文件（当前为 openai.go），内部基于 Eino 的
// eino-ext/components/model/* 实现，但对外只暴露 platform/ai 定义的自研接口，
// 使领域层与具体供应商、与 Eino 解耦。
//
// openai.go 把 ai.GenerateRequest 映射为 Eino 的 *schema.Message 与调用选项，
// 并在返回时回填 ai.Usage 供审计与成本治理。业务包不应直接导入本包。
package provider
