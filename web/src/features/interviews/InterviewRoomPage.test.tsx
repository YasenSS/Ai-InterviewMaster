import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { InterviewSessionResponse } from "@/shared/api/generated/InterviewMasterComponents";
import { api } from "@/shared/api/services";

import { InterviewRoomPage } from "./InterviewRoomPage";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
}));

vi.mock("next/link", () => ({
  default({ href, children }: { href: string; children: ReactNode }) {
    return <a href={href}>{children}</a>;
  },
}));

vi.mock("@/features/auth/AuthGate", () => ({
  useAuth: () => ({ user: { id: "user-1", email: "a@b.c", display_name: "候选人" } }),
}));

vi.mock("@/shared/api/services", () => ({
  api: {
    interview: vi.fn(),
  },
}));

function session(overrides: Partial<InterviewSessionResponse> = {}): InterviewSessionResponse {
  return {
    id: "sess-1",
    title: "模拟面试",
    status: "active",
    primary_language: "中文",
    target_company: "示例公司",
    target_role: "后端",
    resume: { id: "r1", title: "简历" },
    question_count: 2,
    answered_count: 1,
    skipped_count: 0,
    current_ordinal: 1,
    duration_seconds: 40,
    created_at: "2026-08-25T00:00:00.000Z",
    updated_at: "2026-08-25T00:01:00.000Z",
    started_at: "2026-08-25T00:00:00.000Z",
    phase: "deciding",
    agent_mode: "ai",
    policy_version: "v2",
    question_duration_seconds: 180,
    turns: [
      {
        ordinal: 1,
        question: "请介绍你做过的支付系统",
        answer: "我负责对账与幂等，覆盖重复通知。",
        state: "answered",
        time_spent_seconds: 40,
        turn_kind: "main",
      },
      {
        ordinal: 2,
        question: "",
        state: "pending",
        time_spent_seconds: 0,
      },
    ],
    ...overrides,
  };
}

function renderRoom() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <InterviewRoomPage id="sess-1" />
    </QueryClientProvider>,
  );
}

describe("InterviewRoomPage chat transcript", () => {
  beforeEach(() => {
    vi.mocked(api.interview).mockReset();
    Element.prototype.scrollTo = vi.fn();
  });

  it("keeps previous Q&A visible while the interviewer is deciding the next turn", async () => {
    vi.mocked(api.interview).mockResolvedValue(session());
    renderRoom();

    expect(await screen.findByText("请介绍你做过的支付系统")).toBeInTheDocument();
    expect(screen.getByText("我负责对账与幂等，覆盖重复通知。")).toBeInTheDocument();
    expect(screen.getByText(/正在根据你的回答决定下一问/)).toBeInTheDocument();
    expect(screen.getByPlaceholderText("面试官正在思考下一问，回答已保存")).toBeDisabled();
  });

  it("renders interviewer and candidate bubbles with a follow-up badge", async () => {
    vi.mocked(api.interview).mockResolvedValue(
      session({
        phase: "answering",
        current_ordinal: 2,
        answered_count: 1,
        turns: [
          {
            ordinal: 1,
            question: "请介绍你做过的支付系统",
            answer: "我负责对账与幂等。",
            state: "answered",
            time_spent_seconds: 40,
            turn_kind: "main",
          },
          {
            ordinal: 2,
            question: "峰值 QPS 是多少，你怎么压测的？",
            state: "active",
            time_spent_seconds: 0,
            turn_kind: "follow_up",
            difficulty: "medium",
          },
        ],
      }),
    );
    renderRoom();

    expect(await screen.findByText("峰值 QPS 是多少，你怎么压测的？")).toBeInTheDocument();
    expect(screen.getAllByText("请介绍你做过的支付系统")).toHaveLength(1);
    expect(screen.getByText("我负责对账与幂等。")).toBeInTheDocument();
    expect(screen.getByText("追问")).toBeInTheDocument();
    expect(screen.getByText("轮到你回答")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("输入回答，Enter 发送，Shift+Enter 换行")).toBeEnabled();
  });
});
