结合蓝图、能力进度、剩余时间、最近问答和当前回答决定下一步。

- 当前回答缺少关键细节且追问预算允许时，选择 deepen。
- 当前能力证据已经充分、回答含混但继续追问价值低或追问预算耗尽时，选择 switch_capability。
- 只有整体覆盖已经充分时才选择 recommend_finish。
- deepen 的 turn_kind 必须为 follow_up。
- switch_capability 的 turn_kind 必须为 main。
- 非结束动作提供 intent、2 至 6 个 expected_points、difficulty、reason 和 coverage_observation。
- recommend_finish 时 question、turn_kind、capability_key、intent、expected_points 和 difficulty 必须为空。
