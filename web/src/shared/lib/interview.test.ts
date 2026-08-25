import { describe, expect, it } from "vitest";

import {
  draftKey,
  elapsedSeconds,
  isFollowUpTurn,
  isLiveInterviewRoomPath,
  questionRemaining,
  shouldCollapseChatText,
  visibleInterviewTurns,
  QUESTION_SECONDS,
} from "./interview";

describe("interview timing", () => {
  it("derives elapsed time from the authoritative start instant", () => {
    expect(elapsedSeconds("2026-07-29T00:00:00.000Z", Date.parse("2026-07-29T00:02:03.900Z"))).toBe(123);
  });

  it("never returns a negative question countdown", () => {
    expect(questionRemaining(1_000, 1_000)).toBe(QUESTION_SECONDS);
    expect(questionRemaining(1_000, 1_000 + (QUESTION_SECONDS + 9) * 1_000)).toBe(0);
  });

  it("isolates drafts by user, interview and ordinal", () => {
    expect(draftKey("u1", "i1", 2)).toBe("im_draft:u1:i1:2");
    expect(draftKey("u2", "i1", 2)).not.toBe(draftKey("u1", "i1", 2));
  });
});

describe("interview chat transcript", () => {
  it("keeps asked turns and hides questions that have not been issued", () => {
    const turns = [
      { ordinal: 1, question: "介绍项目" },
      { ordinal: 2, question: "追问指标" },
      { ordinal: 3, question: "还没问到" },
    ];
    expect(visibleInterviewTurns(turns, 2).map((turn) => turn.ordinal)).toEqual([1, 2]);
  });

  it("marks follow-up turns for chat labels", () => {
    expect(isFollowUpTurn({ turn_kind: "follow_up" })).toBe(true);
    expect(isFollowUpTurn({ turn_kind: "main" })).toBe(false);
  });

  it("collapses long answers so the thread stays scannable", () => {
    expect(shouldCollapseChatText("短回答")).toBe(false);
    expect(shouldCollapseChatText("a".repeat(361))).toBe(true);
    expect(shouldCollapseChatText("a\n".repeat(9))).toBe(true);
  });

  it("treats only the live room as a chat session, not create or records pages", () => {
    expect(isLiveInterviewRoomPath("/interviews/sess-1")).toBe(true);
    expect(isLiveInterviewRoomPath("/interviews/new")).toBe(false);
    expect(isLiveInterviewRoomPath("/interviews/records")).toBe(false);
    expect(isLiveInterviewRoomPath("/interviews/sess-1/record")).toBe(false);
  });
});
