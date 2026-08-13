-- +goose Up
ALTER TABLE interview_sessions
    ADD COLUMN IF NOT EXISTS current_capability_key text,
    ADD COLUMN IF NOT EXISTS follow_ups_used integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS follow_up_budget integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS interviewer_model text;

ALTER TABLE interview_turns
    ADD COLUMN IF NOT EXISTS turn_kind text NOT NULL DEFAULT 'main',
    ADD COLUMN IF NOT EXISTS parent_turn_id uuid REFERENCES interview_turns(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS capability_key text,
    ADD COLUMN IF NOT EXISTS invocation_id uuid;

CREATE INDEX interview_turns_parent_idx ON interview_turns (parent_turn_id);
ALTER TABLE interview_turns
    DROP CONSTRAINT IF EXISTS interview_turns_kind_chk;
ALTER TABLE interview_turns
    ADD CONSTRAINT interview_turns_kind_chk CHECK (turn_kind IN ('main', 'follow_up'));

CREATE TABLE user_skill_profiles (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    strengths jsonb NOT NULL DEFAULT '[]'::jsonb,
    gaps jsonb NOT NULL DEFAULT '[]'::jsonb,
    notes text NOT NULL DEFAULT '',
    source_session_id uuid REFERENCES interview_sessions(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE public_intel_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company text NOT NULL,
    role text NOT NULL DEFAULT '',
    topic text NOT NULL,
    summary text NOT NULL,
    source_name text NOT NULL,
    source_url text,
    fingerprint text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX public_intel_company_idx ON public_intel_items (lower(company), lower(role));

INSERT INTO public_intel_items (company, role, topic, summary, source_name, fingerprint) VALUES
    ('通用科技公司', '后端工程师', '系统设计', '常见考察可扩展服务、缓存、一致性与故障演练，回答需给出取舍而非标准答案。', 'internal-curated', 'intel-generic-backend-system'),
    ('通用科技公司', '后端工程师', '项目深挖', '面试官通常追问个人贡献、指标口径和复盘，避免只复述团队成果。', 'internal-curated', 'intel-generic-backend-project'),
    ('通用科技公司', '后端工程师', '故障复盘', '线上事故题关注发现路径、止损、根因和防再发，需区分个人动作与团队流程。', 'internal-curated', 'intel-generic-backend-incident'),
    ('通用科技公司', '前端工程师', '性能与体验', '常见问题围绕渲染性能、可访问性和复杂状态管理，需要结合可验证指标。', 'internal-curated', 'intel-generic-frontend-perf'),
    ('通用科技公司', '前端工程师', '工程化', '构建、质量门禁和组件治理常被追问，适合用一次具体改造说明收益。', 'internal-curated', 'intel-generic-frontend-eng'),
    ('通用科技公司', '产品经理', '需求判断', '行为面试强调取舍、用户证据和跨团队对齐，避免空泛“沟通能力”。', 'internal-curated', 'intel-generic-pm-judgment'),
    ('通用科技公司', '产品经理', '指标与实验', '常问如何定义成功、如何处理负面实验结果，以及如何协调研发排期。', 'internal-curated', 'intel-generic-pm-experiment'),
    ('通用科技公司', '数据分析', '指标定义', '常问北极星指标、口径冲突和实验设计，需说明假设与验证方式。', 'internal-curated', 'intel-generic-data-metrics'),
    ('通用科技公司', '算法工程师', '落地权衡', '除模型效果外，还会问延迟、成本和坏例分析，避免只报离线指标。', 'internal-curated', 'intel-generic-algo-tradeoff'),
    ('通用科技公司', '测试开发', '质量策略', '关注风险分层、自动化边界和线上问题回流，需要具体案例。', 'internal-curated', 'intel-generic-qa-strategy'),
    ('通用科技公司', '客户端工程师', '稳定性', '崩溃、兼容性和性能回归是高频追问，适合准备一次完整排查故事。', 'internal-curated', 'intel-generic-client-stability'),
    ('通用科技公司', '运营', '增长与留存', '常把活动目标拆成渠道、转化和复购，并要求解释数据异常。', 'internal-curated', 'intel-generic-ops-growth'),
    ('通用科技公司', '设计', '决策依据', '面试官关注用户证据、约束条件和方案迭代，而不是视觉偏好。', 'internal-curated', 'intel-generic-design-evidence'),
    ('通用科技公司', '项目经理', '推进与升级', '跨团队延期、范围膨胀和升级路径是常见情景题。', 'internal-curated', 'intel-generic-pmgmt-risk'),
    ('通用科技公司', '安全工程师', '威胁建模', '常问如何把业务风险转成控制措施，以及一次真实漏洞的修复闭环。', 'internal-curated', 'intel-generic-sec-threat');

UPDATE app_schema_metadata
SET value = '{"name":"agent-p1p2","version":8}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';

-- +goose Down
DROP TABLE IF EXISTS public_intel_items;
DROP TABLE IF EXISTS user_skill_profiles;
DROP INDEX IF EXISTS interview_turns_parent_idx;
ALTER TABLE interview_turns DROP CONSTRAINT IF EXISTS interview_turns_kind_chk;
ALTER TABLE interview_turns
    DROP COLUMN IF EXISTS invocation_id,
    DROP COLUMN IF EXISTS capability_key,
    DROP COLUMN IF EXISTS parent_turn_id,
    DROP COLUMN IF EXISTS turn_kind;
ALTER TABLE interview_sessions
    DROP COLUMN IF EXISTS interviewer_model,
    DROP COLUMN IF EXISTS follow_up_budget,
    DROP COLUMN IF EXISTS follow_ups_used,
    DROP COLUMN IF EXISTS current_capability_key;
UPDATE app_schema_metadata
SET value = '{"name":"agent-p0","version":7}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';
