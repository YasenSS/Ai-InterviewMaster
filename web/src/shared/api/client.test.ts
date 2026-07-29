import { describe, expect, it } from "vitest";

import { normalizeError } from "./client";

describe("normalizeError", () => {
  it("preserves normalized API errors and request IDs", () => {
    const input = { code: "INVALID_INPUT", message: "请检查输入", requestId: "req-1", fieldErrors: { email: ["格式错误"] } };
    expect(normalizeError(input)).toEqual(input);
  });

  it("hides unknown implementation errors behind a safe network message", () => {
    expect(normalizeError(new Error("database secret"))).toEqual({
      code: "NETWORK_ERROR",
      message: "网络连接失败，请检查网络后重试。",
    });
  });
});
