根据蓝图当前能力点、近几轮问答和本轮回答，决定 action：

- follow_up：针对本轮回答补一个追问。question 必填，capability_key 必须来自蓝图。
- next_capability：不再追问，由系统进入下一题。

约束：
- 不得输出结束面试的动作。
- evidence_fact_ids 只能引用已知事实 ID。
- 若本轮已是追问、预算用尽或回答为空，必须选择 next_capability。
