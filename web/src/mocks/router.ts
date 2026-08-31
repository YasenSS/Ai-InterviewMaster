import { iso, mockClock } from "./clock";
import { apiError, json, noContent } from "./http";
import {
  PARSE_MS,
  PREPARE_MS,
  REPORT_MS,
  authPayload,
  clone,
  finishDecision,
  getMockStore,
  publicUser,
  scheduleDecision,
  seedRunningReport,
  summaryOf,
  tickStore,
  userByToken,
  type MockUser,
} from "./store";

const PUBLIC_PREFIXES = [
  ["POST", "/v1/auth/login"],
  ["POST", "/v1/auth/register"],
  ["POST", "/v1/auth/refresh"],
  ["POST", "/v1/auth/logout"],
  ["GET", "/v1/health"],
  ["GET", "/v1/ready"],
  ["GET", "/v1/metrics"],
  ["PUT", "/v1/uploads/"],
];

export async function handleMockApi(request: Request, segments: string[]) {
  const store = getMockStore();
  tickStore(store);
  const path = `/v1/${segments.join("/")}`;
  const method = request.method.toUpperCase();
  const url = new URL(request.url);
  const token = bearer(request);
  const user = userByToken(store, token);

  if (!isPublic(method, path) && !user) {
    return apiError(401, "UNAUTHORIZED", "登录状态已失效，请重新登录。");
  }

  const body = await readBody(request);

  if (method === "GET" && path === "/v1/health") {
    return json({ status: "ok", service: "interviewmaster-mock", version: "mock", timestamp: iso() });
  }
  if (method === "GET" && path === "/v1/ready") {
    return json({ status: "ok", database: "ok", redis: "ok", timestamp: iso() });
  }
  if (method === "GET" && path === "/v1/metrics") {
    return json({
      requests: 12,
      successes: 11,
      failures: 1,
      retries: 0,
      degraded: 0,
      budget_rejected: 0,
      prompt_tokens: 800,
      completion_tokens: 400,
      total_tokens: 1200,
      estimated_cost_micros: 1200,
      latency_ms_sum: 900,
      structured_first_fail: 0,
      structured_repair_success: 0,
      structured_final_fail: 0,
    });
  }

  if (method === "POST" && path === "/v1/auth/login") {
    const email = String(body.email ?? "").trim().toLowerCase();
    if (!email) return apiError(400, "INVALID_INPUT", "请输入邮箱。", { email: ["请输入邮箱"] });
    let account = store.users.find((item) => item.email === email);
    if (!account) {
      account = {
        id: `user-${store.nextSeq++}`,
        email,
        display_name: email.split("@")[0] || "候选人",
        password: String(body.password ?? ""),
      };
      store.users.push(account);
    }
    return json(authPayload(store, account));
  }
  if (method === "POST" && path === "/v1/auth/register") {
    const email = String(body.email ?? "").trim().toLowerCase();
    if (!String(body.invite_code ?? "").trim()) {
      return apiError(403, "INVITE_CODE_INVALID", "邀请码无效或已失效。", { invite_code: ["请输入邀请码"] });
    }
    if (store.users.some((item) => item.email === email)) {
      return apiError(409, "EMAIL_TAKEN", "该邮箱已被使用，请直接登录或更换邮箱。", { email: ["该邮箱已被使用"] });
    }
    const account: MockUser = {
      id: `user-${store.nextSeq++}`,
      email,
      display_name: String(body.display_name ?? "新用户").trim() || "新用户",
      password: String(body.password ?? ""),
    };
    store.users.push(account);
    return json(authPayload(store, account));
  }
  if (method === "POST" && path === "/v1/auth/refresh") {
    const fallback = store.users[0];
    if (!fallback) return apiError(401, "UNAUTHORIZED", "登录状态已失效，请重新登录。");
    return json(authPayload(store, user ?? fallback));
  }
  if (method === "POST" && path === "/v1/auth/logout") return noContent();
  if (method === "POST" && path === "/v1/auth/change-password") return noContent();

  if (method === "GET" && path === "/v1/me") return json(publicUser(user!));
  if (method === "PATCH" && path === "/v1/me") {
    user!.display_name = String(body.display_name ?? user!.display_name).trim() || user!.display_name;
    return json(publicUser(user!));
  }
  if (method === "GET" && path === "/v1/me/export") {
    return json({
      exported_at: iso(),
      user: publicUser(user!),
      resumes: store.resumes,
      interviews: store.interviews.map(summaryOf),
      reports: Object.values(store.reports),
      tasks: [],
      skill_profile: store.skillProfile,
      model_invocations: [],
    });
  }
  if (method === "POST" && path === "/v1/me/delete") return noContent();
  if (method === "GET" && path === "/v1/me/skill-profile") return json(clone(store.skillProfile));
  if (method === "PATCH" && path === "/v1/me/skill-profile") {
    store.skillProfile = {
      strengths: Array.isArray(body.strengths) ? body.strengths : store.skillProfile.strengths,
      gaps: Array.isArray(body.gaps) ? body.gaps : store.skillProfile.gaps,
      notes: typeof body.notes === "string" ? body.notes : store.skillProfile.notes,
      source_session_id: store.skillProfile.source_session_id,
      updated_at: iso(),
    };
    return json(clone(store.skillProfile));
  }
  if (method === "DELETE" && path === "/v1/me/skill-profile") {
    store.skillProfile = { strengths: [], gaps: [], notes: "", updated_at: iso() };
    return noContent();
  }

  if (method === "GET" && path === "/v1/resumes") {
    const page = Number(url.searchParams.get("page") ?? 1);
    const pageSize = Number(url.searchParams.get("page_size") ?? 50);
    return json(paginate(store.resumes.map(toResumeSummary), page, pageSize));
  }
  const resumeMatch = path.match(/^\/v1\/resumes\/([^/]+)$/);
  if (resumeMatch && method === "GET") {
    const resume = store.resumes.find((item) => item.id === resumeMatch[1]);
    if (!resume) return apiError(404, "NOT_FOUND", "请求的内容不存在或已被删除。");
    return json(clone(resume));
  }
  if (method === "POST" && path === "/v1/resumes/uploads") {
    const id = `resume-${store.nextSeq++}`;
    const versionId = `${id}-v1`;
    const objectId = `object-${id}`;
    const title = String(body.title ?? "未命名简历").trim() || "未命名简历";
    store.resumes.unshift({
      id,
      title,
      status: "pending",
      version_id: versionId,
      file_name: String(body.file_name ?? "resume.pdf"),
      created_at: iso(),
      updated_at: iso(),
      version_no: 1,
      content_type: String(body.content_type ?? "application/pdf"),
      size_bytes: Number(body.size_bytes ?? 0),
      uploaded_at: iso(),
      facts: [],
    });
    store.uploads[objectId] = { resumeId: id, versionId };
    return json({
      resume_id: id,
      version_id: versionId,
      upload_url: `/api/v1/uploads/${objectId}`,
      upload_headers: { "x-mock-upload": "1" },
      expires_at: iso(15 * 60_000),
    });
  }
  const completeMatch = path.match(/^\/v1\/resumes\/([^/]+)\/versions\/([^/]+)\/complete$/);
  if (completeMatch && method === "POST") {
    const resume = store.resumes.find((item) => item.id === completeMatch[1]);
    if (!resume) return apiError(404, "NOT_FOUND", "请求的内容不存在或已被删除。");
    resume.status = "processing";
    resume.updated_at = iso();
    store.agenda.resumeReadyAt[resume.id] = mockClock.now() + PARSE_MS;
    return json(clone(resume));
  }
  const reparseMatch = path.match(/^\/v1\/resumes\/([^/]+)\/reparse$/);
  if (reparseMatch && method === "POST") {
    const resume = store.resumes.find((item) => item.id === reparseMatch[1]);
    if (!resume) return apiError(404, "NOT_FOUND", "请求的内容不存在或已被删除。");
    resume.status = "processing";
    resume.parse_error = undefined;
    resume.updated_at = iso();
    store.agenda.resumeReadyAt[resume.id] = mockClock.now() + PARSE_MS;
    return json(clone(resume));
  }
  if (method === "PUT" && path.startsWith("/v1/uploads/")) return noContent();

  if (method === "GET" && path === "/v1/interviews") {
    const page = Number(url.searchParams.get("page") ?? 1);
    const pageSize = Number(url.searchParams.get("page_size") ?? 50);
    const status = url.searchParams.get("status") ?? undefined;
    const items = store.interviews.filter((item) => !status || item.status === status).map(summaryOf);
    return json(paginate(items, page, pageSize));
  }
  if (method === "POST" && path === "/v1/interviews") {
    const resume = store.resumes.find((item) => item.id === body.resume_id && item.status === "completed");
    if (!resume) return apiError(400, "RESUME_REQUIRED", "请选择一份已解析的简历。");
    const id = `interview-${store.nextSeq++}`;
    const session = {
      id,
      title: `${String(body.target_company ?? "目标公司")} · ${String(body.primary_language ?? "Java")} 后端面试`,
      status: "preparing",
      primary_language: String(body.primary_language ?? "Java"),
      target_company: String(body.target_company ?? "目标公司"),
      target_role: "后端开发",
      resume: { id: resume.id, title: resume.title },
      question_count: 0,
      answered_count: 0,
      skipped_count: 0,
      current_ordinal: 0,
      duration_seconds: 0,
      created_at: iso(),
      updated_at: iso(),
      phase: "preparing",
      agent_mode: "ai",
      policy_version: "v2",
      question_duration_seconds: Number(body.question_duration_seconds ?? 180),
      turns: [],
    };
    store.interviews.unshift(session);
    store.agenda.interviewReadyAt[id] = mockClock.now() + PREPARE_MS;
    return json(clone(session));
  }

  const interviewMatch = path.match(/^\/v1\/interviews\/([^/]+)(?:\/(.*))?$/);
  if (interviewMatch) {
    const session = store.interviews.find((item) => item.id === interviewMatch[1]);
    if (!session) return apiError(404, "NOT_FOUND", "请求的内容不存在或已被删除。");
    const rest = interviewMatch[2] ?? "";

    if (method === "GET" && rest === "") return json(clone(session));
    if (method === "POST" && rest === "preparation/retry") {
      session.status = "preparing";
      session.phase = "preparing";
      session.operation = undefined;
      session.updated_at = iso();
      store.agenda.interviewReadyAt[session.id] = mockClock.now() + PREPARE_MS;
      return json(clone(session));
    }
    if (method === "POST" && rest === "next-turn/retry") {
      session.phase = "deciding";
      session.operation = undefined;
      scheduleDecision(store, session);
      return json(clone(session));
    }
    if (method === "POST" && rest === "next-turn/fallback") {
      finishDecision(store, session, "rule");
      return json(clone(session));
    }
    if (method === "POST" && rest === "complete") {
      session.status = "completed";
      session.phase = "completed";
      session.completed_at = iso();
      session.updated_at = iso();
      seedRunningReport(store, session);
      return json(clone(session));
    }
    const answerMatch = rest.match(/^turns\/(\d+)\/answer$/);
    if (method === "PUT" && answerMatch) {
      const ordinal = Number(answerMatch[1]);
      const turn = session.turns.find((item) => item.ordinal === ordinal);
      if (!turn) return apiError(404, "NOT_FOUND", "当前题目不存在。");
      turn.answer = String(body.answer ?? "").trim();
      turn.state = "answered";
      turn.answered_at = iso();
      turn.time_spent_seconds = 40;
      session.answered_count += 1;
      session.updated_at = iso();
      scheduleDecision(store, session);
      return json(clone(session));
    }
    const skipMatch = rest.match(/^turns\/(\d+)\/skip$/);
    if (method === "POST" && skipMatch) {
      const ordinal = Number(skipMatch[1]);
      const turn = session.turns.find((item) => item.ordinal === ordinal);
      if (!turn) return apiError(404, "NOT_FOUND", "当前题目不存在。");
      turn.state = "skipped";
      turn.skipped_at = iso();
      session.skipped_count += 1;
      session.updated_at = iso();
      scheduleDecision(store, session);
      return json(clone(session));
    }
    if (method === "GET" && rest === "report") {
      const report = store.reports[session.id];
      if (!report && session.status !== "completed") {
        return apiError(409, "NOT_READY", "面试尚未完成，暂无评估。");
      }
      if (!report) {
        seedRunningReport(store, session);
        return json(clone(store.reports[session.id]));
      }
      return json(clone(report));
    }
    if (method === "POST" && rest === "report/retry") {
      seedRunningReport(store, session);
      store.agenda.reportReadyAt[session.id] = mockClock.now() + REPORT_MS;
      return json(clone(store.reports[session.id]));
    }
  }

  return apiError(404, "NOT_FOUND", `Mock 未覆盖 ${method} ${path}`);
}

function bearer(request: Request) {
  const header = request.headers.get("authorization") ?? "";
  const match = header.match(/^Bearer\s+(.+)$/i);
  return match?.[1] ?? null;
}

function isPublic(method: string, path: string) {
  return PUBLIC_PREFIXES.some(([itemMethod, prefix]) => itemMethod === method && path.startsWith(prefix));
}

async function readBody(request: Request): Promise<Record<string, unknown>> {
  if (request.method === "GET" || request.method === "HEAD") return {};
  const text = await request.text();
  if (!text) return {};
  try {
    return JSON.parse(text) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function paginate<T>(items: T[], page: number, pageSize: number) {
  const start = Math.max(0, (page - 1) * pageSize);
  return {
    items: items.slice(start, start + pageSize),
    page,
    page_size: pageSize,
    total: items.length,
  };
}

function toResumeSummary(resume: ReturnType<typeof getMockStore>["resumes"][number]) {
  return {
    id: resume.id,
    title: resume.title,
    status: resume.status,
    version_id: resume.version_id,
    file_name: resume.file_name,
    created_at: resume.created_at,
    updated_at: resume.updated_at,
  };
}
