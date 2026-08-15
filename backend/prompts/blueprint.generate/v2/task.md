根据输入生成 InterviewBlueprintV2。

- schema_version 固定为 v2，mode 固定为 standard。
- min_turns=8、target_turns=12、max_turns=16。
- time_budget_minutes=30。
- max_follow_up_depth=2、max_follow_ups_total=4。
- 生成 4 至 8 个能力点，权重总和为 100。
- 每个能力点包含唯一 key、中文 label、target_evidence、difficulty_curve 和 2 至 6 条 Rubric。
- 覆盖主语言后端基础、项目深挖、系统设计/工程能力、故障与问题解决；可根据材料增加协作或动机。
