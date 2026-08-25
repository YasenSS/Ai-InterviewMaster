import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { mockClock } from "./clock";
import { handleMockApi } from "./router";
import { DECIDE_MS, PREPARE_MS, resetMockStore } from "./store";

let now = 1_700_000_000_000;

async function call(
  method: string,
  path: string,
  init: { token?: string; body?: unknown; query?: string } = {},
) {
  const segments = path.replace(/^\//, "").split("/").filter(Boolean);
  const request = new Request(`http://localhost/api/v1/${segments.join("/")}${init.query ?? ""}`, {
    method,
    headers: {
      "Content-Type": "application/json",
      ...(init.token ? { Authorization: `Bearer ${init.token}` } : {}),
    },
    body: init.body ? JSON.stringify(init.body) : undefined,
  });
  const response = await handleMockApi(request, segments);
  const text = await response.text();
  return {
    status: response.status,
    body: text ? (JSON.parse(text) as Record<string, unknown>) : undefined,
  };
}

async function login() {
  const response = await call("POST", "auth/login", { body: { email: "mock@local.dev", password: "any" } });
  return String((response.body as { access_token: string }).access_token);
}

describe("frontend mock API", () => {
  beforeEach(() => {
    now = 1_700_000_000_000;
    mockClock.now = () => now;
    resetMockStore();
  });

  afterEach(() => {
    mockClock.now = () => Date.now();
  });

  it("serves health without a backend", async () => {
    const response = await call("GET", "health");
    expect(response.status).toBe(200);
    expect(response.body).toMatchObject({ status: "ok", version: "mock" });
  });

  it("logs in and returns the current user", async () => {
    const token = await login();
    const me = await call("GET", "me", { token });
    expect(me.status).toBe(200);
    expect(me.body).toMatchObject({ email: "mock@local.dev", display_name: "演示用户" });
  });

  it("lists seeded resumes and interview records", async () => {
    const token = await login();
    const resumes = await call("GET", "resumes", { token });
    expect(resumes.status).toBe(200);
    expect((resumes.body as { items: unknown[] }).items.length).toBeGreaterThanOrEqual(3);

    const interviews = await call("GET", "interviews", { token, query: "?page=1&page_size=20" });
    expect((interviews.body as { total: number }).total).toBeGreaterThanOrEqual(6);
  });

  it("creates an interview then advances from preparing to the first question", async () => {
    const token = await login();
    const created = await call("POST", "interviews", {
      token,
      body: { resume_id: "resume-parsed", primary_language: "Go", target_company: "美团" },
    });
    expect(created.status).toBe(200);
    expect(created.body).toMatchObject({ status: "preparing", phase: "preparing" });
    const id = String((created.body as { id: string }).id);

    now += PREPARE_MS + 10;
    const ready = await call("GET", `interviews/${id}`, { token });
    expect(ready.body).toMatchObject({ status: "active", phase: "answering", current_ordinal: 1 });
    expect((ready.body as { turns: Array<{ question: string }> }).turns[0]?.question).toContain("后端项目");
  });

  it("keeps history after an answer and then issues a follow-up", async () => {
    const token = await login();
    const answered = await call("PUT", "interviews/interview-live/turns/2/answer", {
      token,
      body: { answer: "我对账差异量和重复入账笔数做了前后对比。" },
    });
    expect(answered.body).toMatchObject({ phase: "deciding" });
    expect((answered.body as { turns: Array<{ answer?: string }> }).turns[0]?.answer).toBeTruthy();

    now += DECIDE_MS + 10;
    const next = await call("GET", "interviews/interview-live", { token });
    expect(next.body).toMatchObject({ phase: "answering", current_ordinal: 3 });
    expect((next.body as { turns: unknown[] }).turns).toHaveLength(3);
  });

  it("retries a failed next-turn with the fallback question", async () => {
    const token = await login();
    const fallback = await call("POST", "interviews/interview-decision-failed/next-turn/fallback", { token });
    expect(fallback.status).toBe(200);
    expect(fallback.body).toMatchObject({ phase: "answering", agent_mode: "rule" });
    expect((fallback.body as { turns: unknown[] }).turns.length).toBeGreaterThan(1);
  });

  it("retries a failed report", async () => {
    const token = await login();
    const retry = await call("POST", "interviews/interview-report-failed/report/retry", { token });
    expect(retry.body).toMatchObject({ status: "running" });
  });
});
