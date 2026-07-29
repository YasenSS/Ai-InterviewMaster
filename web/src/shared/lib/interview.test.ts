import { describe, expect, it } from "vitest";

import { draftKey, elapsedSeconds, questionRemaining, QUESTION_SECONDS } from "./interview";

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
