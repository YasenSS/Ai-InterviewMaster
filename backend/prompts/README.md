# Prompts

本目录存放所有 Prompt 模板与版本元数据，与代码分离、可版本化、可回滚。

## 目录约定

```text
prompts/
  <prompt_key>/            # 用点分命名，如 question.generate
    <version>/             # 如 v1、v2
      system.md            # 系统规则（与任务/证据/用户输入分层分隔，防注入）
      task.md              # 任务说明
      meta.yaml            # 模型、温度、max_tokens、输出 JSON Schema 引用
```

## 规划中的 prompt_key

| prompt_key | 用途 | 调用节点 |
| --- | --- | --- |
| `resume.extract` | 简历全文 → 结构化事实 JSON（带 source_span） | Worker resumeparse |
| `blueprint.generate` | 简历事实+语言+公司 → InterviewBlueprint | 面试后台准备 |
| `question.generate` | 蓝图+简历+语言+公司 → 候选问题/考察点 | 面试后台准备 |
| `interview.followup` | 蓝图+近几轮 → 追问/切换能力/结束 | 面试流程决策节点 |
| `evaluation.critique` | 题目+回答+证据 → 五维评分+点评 | 评分 logic |

## 纪律

- 每个版本一次冻结，升级 = 新增 version 目录，不改旧版本（可回溯）。
- 用户提供的简历、目标公司和回答永远注入「数据区」，用明确分隔符与指令隔离，不混排。
- 输出必须配 JSON Schema，进入 Go struct 校验；失败只允许一次修复重试。
- 每次调用记录 `prompt_key + prompt_version` 到 `model_invocations`。
