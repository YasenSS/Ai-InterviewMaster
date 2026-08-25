import type {
  InterviewReportResponse,
  InterviewSessionResponse,
  InterviewTurnResponse,
  ResumeDetailResponse,
  ResumeFactResponse,
  SkillProfileResponse,
  UserResponse,
} from "@/shared/api/generated/InterviewMasterComponents";

import { iso, mockClock } from "./clock";

export const PREPARE_MS = 1600;
export const DECIDE_MS = 1400;
export const PARSE_MS = 1600;
export const REPORT_MS = 1800;

export type MockUser = UserResponse & { password: string };

export const MOCK_QUESTIONS: Array<Pick<InterviewTurnResponse, "question" | "turn_kind" | "difficulty" | "capability_key" | "intent">> = [
  {
    turn_kind: "main",
    difficulty: "medium",
    capability_key: "project_ownership",
    intent: "核实项目职责边界",
    question: "请选一个你主导过的后端项目，说明你负责的模块、关键约束，以及你做过的最重要技术决策。",
  },
  {
    turn_kind: "follow_up",
    difficulty: "medium",
    capability_key: "project_ownership",
    intent: "追问可量化结果",
    question: "你刚才提到的优化，具体指标是怎么测量的？有没有对照实验或回滚预案？",
  },
  {
    turn_kind: "main",
    difficulty: "medium",
    capability_key: "concurrency",
    intent: "考察并发与一致性",
    question: "如果支付回调会重复到达，你如何设计接口保证只成功入账一次？",
  },
  {
    turn_kind: "follow_up",
    difficulty: "hard",
    capability_key: "concurrency",
    intent: "追问失败场景",
    question: "如果幂等键写入成功但下游扣款超时，你下一步怎么做？",
  },
];

type Agenda = {
  resumeReadyAt: Record<string, number>;
  interviewReadyAt: Record<string, number>;
  nextTurnAt: Record<string, number>;
  reportReadyAt: Record<string, number>;
};

export type MockStore = {
  users: MockUser[];
  tokens: Record<string, string>;
  resumes: ResumeDetailResponse[];
  interviews: InterviewSessionResponse[];
  reports: Record<string, InterviewReportResponse>;
  skillProfile: SkillProfileResponse;
  uploads: Record<string, { resumeId: string; versionId: string }>;
  agenda: Agenda;
  nextSeq: number;
};

const globalStore = globalThis as typeof globalThis & { __imMockStore?: MockStore };

export function getMockStore(): MockStore {
  if (!globalStore.__imMockStore) globalStore.__imMockStore = createSeededStore();
  return globalStore.__imMockStore;
}

export function resetMockStore(store = createSeededStore()) {
  globalStore.__imMockStore = store;
  return store;
}

export function clone<T>(value: T): T {
  return structuredClone(value);
}

export function issueToken(store: MockStore, userId: string) {
  const token = `mock-token-${userId}-${store.nextSeq++}`;
  store.tokens[token] = userId;
  return token;
}

export function userByToken(store: MockStore, token: string | null) {
  if (!token) return undefined;
  const userId = store.tokens[token];
  return store.users.find((user) => user.id === userId);
}

export function publicUser(user: MockUser): UserResponse {
  return { id: user.id, email: user.email, display_name: user.display_name };
}

export function authPayload(store: MockStore, user: MockUser) {
  return {
    access_token: issueToken(store, user.id),
    token_type: "Bearer",
    expires_in: 3600,
    user: publicUser(user),
  };
}

export function summaryOf(session: InterviewSessionResponse) {
  return {
    id: session.id,
    title: session.title,
    status: session.status,
    primary_language: session.primary_language,
    target_company: session.target_company,
    target_role: session.target_role,
    resume: session.resume,
    question_count: session.question_count,
    answered_count: session.answered_count,
    skipped_count: session.skipped_count,
    current_ordinal: session.current_ordinal,
    overall_score: session.overall_score,
    started_at: session.started_at,
    completed_at: session.completed_at,
    duration_seconds: session.duration_seconds,
    created_at: session.created_at,
    updated_at: session.updated_at,
  };
}

export function tickStore(store: MockStore) {
  const now = mockClock.now();
  for (const resume of store.resumes) {
    const readyAt = store.agenda.resumeReadyAt[resume.id];
    if (resume.status === "processing" && readyAt && now >= readyAt) {
      completeResume(resume);
      delete store.agenda.resumeReadyAt[resume.id];
    }
  }
  for (const session of store.interviews) {
    const prepareAt = store.agenda.interviewReadyAt[session.id];
    if ((session.status === "preparing" || session.phase === "preparing") && prepareAt && now >= prepareAt) {
      activateInterview(store, session);
      delete store.agenda.interviewReadyAt[session.id];
    }
    const nextAt = store.agenda.nextTurnAt[session.id];
    if (session.phase === "deciding" && nextAt && now >= nextAt) {
      finishDecision(store, session);
      delete store.agenda.nextTurnAt[session.id];
    }
    const reportAt = store.agenda.reportReadyAt[session.id];
    const report = store.reports[session.id];
    if (report && ["pending", "running"].includes(report.status) && reportAt && now >= reportAt) {
      completeReport(store, session);
      delete store.agenda.reportReadyAt[session.id];
    }
  }
}

export function completeResume(resume: ResumeDetailResponse) {
  resume.status = "completed";
  resume.processed_at = iso();
  resume.updated_at = iso();
  resume.parse_error = undefined;
  if (!resume.facts.length) resume.facts = factsFor(resume.title);
}

export function activateInterview(store: MockStore, session: InterviewSessionResponse) {
  const turn = makeTurn(1, MOCK_QUESTIONS[0]!);
  session.status = "active";
  session.phase = "answering";
  session.started_at = iso();
  session.updated_at = iso();
  session.current_ordinal = 1;
  session.question_count = 1;
  session.turns = [turn];
  session.operation = undefined;
  session.agent_mode = "ai";
}

export function scheduleDecision(store: MockStore, session: InterviewSessionResponse) {
  session.phase = "deciding";
  session.updated_at = iso();
  store.agenda.nextTurnAt[session.id] = mockClock.now() + DECIDE_MS;
}

export function finishDecision(store: MockStore, session: InterviewSessionResponse, mode: "ai" | "rule" = "ai") {
  const answered = session.turns.filter((turn) => Boolean(turn.answer) || turn.state === "skipped").length;
  if (answered >= 3) {
    completeInterview(store, session);
    return;
  }
  const nextOrdinal = session.current_ordinal + 1;
  const template = MOCK_QUESTIONS[Math.min(nextOrdinal - 1, MOCK_QUESTIONS.length - 1)]!;
  const turn = makeTurn(nextOrdinal, template, mode);
  session.turns.push(turn);
  session.current_ordinal = nextOrdinal;
  session.question_count = session.turns.length;
  session.phase = "answering";
  session.status = "active";
  session.updated_at = iso();
  session.operation = undefined;
  if (mode === "rule") session.agent_mode = "rule";
}

export function completeInterview(store: MockStore, session: InterviewSessionResponse) {
  session.status = "completed";
  session.phase = "completed";
  session.completed_at = iso();
  session.updated_at = iso();
  session.duration_seconds = Math.max(60, Math.floor((mockClock.now() - Date.parse(session.started_at ?? session.created_at)) / 1000));
  session.completion_reason = "coverage_complete";
  session.overall_score = 86;
  seedRunningReport(store, session);
}

export function seedRunningReport(store: MockStore, session: InterviewSessionResponse) {
  store.reports[session.id] = {
    id: `report-${session.id}`,
    session_id: session.id,
    status: "running",
    overall_score: 0,
    strengths: [],
    improvements: [],
    next_steps: [],
    quality_passed: false,
    degraded: false,
    turns: [],
    created_at: iso(),
    updated_at: iso(),
    operation: { type: "report", status: "running", retryable: true },
  };
  store.agenda.reportReadyAt[session.id] = mockClock.now() + REPORT_MS;
}

export function completeReport(store: MockStore, session: InterviewSessionResponse) {
  const turns = session.turns.filter((turn) => Boolean(turn.question)).map((turn, index) => ({
    ordinal: turn.ordinal,
    question: turn.question,
    answer: turn.answer,
    score: 90 - index * 4,
    critique: turn.answer
      ? "结构清楚，建议把约束、取舍和验证结果再写具体一些。"
      : "本轮缺少有效回答，分数反映的是证据不足而不是能力不足。",
    golden_answer: "先交代场景和约束，再说明行动、验证方式和可复用结论。",
    evidence: turn.answer ? ["回答中包含具体模块职责", "提到了可验证的结果"] : ["本题未作答"],
  }));
  const overall = Math.round(turns.reduce((sum, turn) => sum + turn.score, 0) / Math.max(turns.length, 1));
  store.reports[session.id] = {
    id: `report-${session.id}`,
    session_id: session.id,
    status: "completed",
    overall_score: overall,
    strengths: ["能从真实项目出发说明职责边界", "对幂等和失败处理有基本意识"],
    improvements: ["量化结果时补上测量方法和对照", "超时与回滚路径可以讲得更完整"],
    next_steps: ["用同一项目再练一轮追问", "专门练习把决策依据讲清楚"],
    quality_passed: overall >= 70,
    degraded: false,
    turns,
    created_at: store.reports[session.id]?.created_at ?? iso(),
    updated_at: iso(),
  };
  session.overall_score = overall;
}

export function makeTurn(
  ordinal: number,
  template: (typeof MOCK_QUESTIONS)[number],
  generationMode: "ai" | "rule" = "ai",
): InterviewTurnResponse {
  return {
    ordinal,
    question: template.question ?? "",
    state: "active",
    time_spent_seconds: 0,
    turn_kind: template.turn_kind,
    capability_key: template.capability_key,
    intent: template.intent,
    difficulty: template.difficulty,
    generation_mode: generationMode,
    started_at: iso(),
  };
}

export function factsFor(title: string): ResumeFactResponse[] {
  return [
    { type: "personal", key: "姓名", value: "陈思远", excerpt: `${title}中的个人信息`, confidence: 0.93 },
    { type: "education", key: "学历", value: "计算机科学 本科", excerpt: "计算机科学与技术，本科", confidence: 0.9 },
    { type: "work", key: "最近经历", value: "支付科技公司 · 后端开发 · 2022 至今", excerpt: "负责交易与对账相关服务", confidence: 0.88 },
    { type: "project", key: "支付对账", value: "主导日终对账与幂等扣款改造", excerpt: "将重复通知导致的重复入账降到接近零", confidence: 0.86 },
    { type: "skill", key: "技术栈", value: "Java / Go / PostgreSQL / Redis", excerpt: "熟悉高并发记账与一致性方案", confidence: 0.84 },
  ];
}

function createSeededStore(): MockStore {
  const demo: MockUser = {
    id: "user-demo",
    email: "mock@local.dev",
    display_name: "演示用户",
    password: "mockpass123",
  };
  const parsed = seedResume("resume-parsed", "陈思远 · 后端简历", "completed", "chen-siyuan.pdf");
  parsed.facts = factsFor(parsed.title);
  const parsing = seedResume("resume-parsing", "实习简历（解析中）", "processing", "intern.docx");
  const failed = seedResume("resume-failed", "扫描件简历", "failed", "scan.pdf");
  failed.parse_error = "文字层不完整，Mock 中可重新上传一份新文件。";

  const live = seedSession("interview-live", parsed, {
    status: "active",
    phase: "answering",
    current_ordinal: 2,
    answered_count: 1,
    started_at: iso(-12 * 60_000),
  });
  live.turns = [
    {
      ...makeTurn(1, MOCK_QUESTIONS[0]!),
      answer: "我在支付对账服务里负责幂等键和日终差异排查，把重复入账从每天几十笔降到接近零。",
      state: "answered",
      answered_at: iso(-8 * 60_000),
      time_spent_seconds: 95,
    },
    makeTurn(2, MOCK_QUESTIONS[1]!),
  ];
  live.question_count = 2;

  const deciding = seedSession("interview-deciding", parsed, {
    status: "active",
    phase: "deciding",
    current_ordinal: 1,
    answered_count: 1,
    started_at: iso(-6 * 60_000),
  });
  deciding.turns = [
    {
      ...makeTurn(1, MOCK_QUESTIONS[0]!),
      answer: "我负责对账批处理的分片和重试，核心是避免同一笔差异被两次修复。",
      state: "answered",
      answered_at: iso(-30_000),
      time_spent_seconds: 70,
    },
  ];

  const decisionFailed = seedSession("interview-decision-failed", parsed, {
    status: "active",
    phase: "decision_failed",
    current_ordinal: 1,
    answered_count: 1,
    started_at: iso(-9 * 60_000),
  });
  decisionFailed.turns = deciding.turns.map((turn) => clone(turn));
  decisionFailed.operation = {
    type: "next_turn",
    status: "failed",
    error_code: "AI_OUTPUT_INVALID",
    error_summary: "下一轮问题生成失败。回答已保存，可以重试或使用系统兜底问题继续。",
    retryable: true,
  };

  const preparing = seedSession("interview-preparing", parsed, {
    status: "preparing",
    phase: "preparing",
    current_ordinal: 0,
    answered_count: 0,
  });

  const failedPrepare = seedSession("interview-failed", parsed, {
    status: "failed",
    phase: "failed",
    current_ordinal: 0,
    answered_count: 0,
  });
  failedPrepare.operation = {
    type: "prepare",
    status: "failed",
    error_code: "PROVIDER_UNAVAILABLE",
    error_summary: "准备面试时模型暂时不可用。简历和历史记录没有受到影响。",
    retryable: true,
  };

  const completed = seedSession("interview-completed", parsed, {
    status: "completed",
    phase: "completed",
    current_ordinal: 3,
    answered_count: 3,
    started_at: iso(-2 * 60 * 60_000),
    completed_at: iso(-90 * 60_000),
    duration_seconds: 18 * 60,
    overall_score: 88,
  });
  completed.turns = [
    { ...makeTurn(1, MOCK_QUESTIONS[0]!), answer: "我对账服务做了分片和幂等键，减少重复入账。", state: "answered", time_spent_seconds: 80, answered_at: iso(-110 * 60_000) },
    { ...makeTurn(2, MOCK_QUESTIONS[1]!), answer: "用对账差异量和重复入账笔数做前后对比，并保留一键回滚开关。", state: "answered", time_spent_seconds: 70, answered_at: iso(-100 * 60_000) },
    { ...makeTurn(3, MOCK_QUESTIONS[2]!), answer: "用业务幂等键加唯一约束，回调先落库再异步扣款。", state: "answered", time_spent_seconds: 90, answered_at: iso(-95 * 60_000) },
  ];
  completed.question_count = 3;
  completed.completion_reason = "coverage_complete";

  const reportFailed = seedSession("interview-report-failed", parsed, {
    status: "completed",
    phase: "completed",
    current_ordinal: 2,
    answered_count: 2,
    started_at: iso(-3 * 60 * 60_000),
    completed_at: iso(-2 * 60 * 60_000),
    duration_seconds: 12 * 60,
  });
  reportFailed.turns = completed.turns.slice(0, 2).map((turn, index) => ({ ...clone(turn), ordinal: index + 1 }));
  reportFailed.question_count = 2;

  const store: MockStore = {
    users: [demo],
    tokens: {},
    resumes: [parsed, parsing, failed],
    interviews: [live, deciding, decisionFailed, preparing, failedPrepare, completed, reportFailed],
    reports: {},
    skillProfile: {
      strengths: ["能讲清项目职责和约束", "对幂等有实操经验"],
      gaps: ["量化结果时缺少测量方法"],
      notes: "Mock 画像，保存后会立刻反映在本页。",
      source_session_id: completed.id,
      updated_at: iso(-80 * 60_000),
    },
    uploads: {},
    agenda: {
      resumeReadyAt: { [parsing.id]: mockClock.now() + PARSE_MS },
      interviewReadyAt: { [preparing.id]: mockClock.now() + PREPARE_MS },
      nextTurnAt: { [deciding.id]: mockClock.now() + DECIDE_MS },
      reportReadyAt: {},
    },
    nextSeq: 1,
  };

  completeReport(store, completed);
  store.reports[reportFailed.id] = {
    id: "report-interview-report-failed",
    session_id: reportFailed.id,
    status: "failed",
    overall_score: 0,
    strengths: [],
    improvements: [],
    next_steps: [],
    quality_passed: false,
    degraded: false,
    error_code: "AI_OUTPUT_INVALID",
    error_summary: "评估生成失败。完整问答没有丢失，可以重试。",
    operation: { type: "report", status: "failed", error_code: "AI_OUTPUT_INVALID", error_summary: "评估生成失败。", retryable: true },
    turns: [],
    created_at: iso(-2 * 60 * 60_000),
    updated_at: iso(-2 * 60 * 60_000),
  };
  return store;
}

function seedResume(id: string, title: string, status: string, fileName: string): ResumeDetailResponse {
  return {
    id,
    title,
    status,
    version_id: `${id}-v1`,
    file_name: fileName,
    created_at: iso(-5 * 24 * 60 * 60_000),
    updated_at: iso(-2 * 60 * 60_000),
    version_no: 1,
    content_type: fileName.endsWith(".pdf") ? "application/pdf" : "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    size_bytes: 128_000,
    uploaded_at: iso(-5 * 24 * 60 * 60_000),
    facts: [],
  };
}

function seedSession(
  id: string,
  resume: ResumeDetailResponse,
  extra: Partial<InterviewSessionResponse>,
): InterviewSessionResponse {
  return {
    id,
    title: `${extra.target_company ?? "字节跳动"} · Java 后端面试`,
    status: extra.status ?? "active",
    primary_language: extra.primary_language ?? "Java",
    target_company: extra.target_company ?? "字节跳动",
    target_role: "后端开发",
    resume: { id: resume.id, title: resume.title },
    question_count: extra.question_count ?? 0,
    answered_count: extra.answered_count ?? 0,
    skipped_count: extra.skipped_count ?? 0,
    current_ordinal: extra.current_ordinal ?? 0,
    overall_score: extra.overall_score,
    started_at: extra.started_at,
    completed_at: extra.completed_at,
    duration_seconds: extra.duration_seconds ?? 0,
    created_at: iso(-24 * 60 * 60_000),
    updated_at: iso(-60 * 60_000),
    phase: extra.phase ?? "answering",
    agent_mode: "ai",
    policy_version: "v2",
    completion_reason: extra.completion_reason,
    operation: extra.operation,
    question_duration_seconds: 180,
    turns: [],
  };
}
