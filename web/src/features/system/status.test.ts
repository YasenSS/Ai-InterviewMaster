import { describe, expect, it } from "vitest";

import { summarizeStatus } from "./status";

describe("summarizeStatus", () => {
  it("reports ready only when API and dependencies are available", () => {
    expect(summarizeStatus(true, true)).toBe("ready");
    expect(summarizeStatus(true, false)).toBe("partial");
    expect(summarizeStatus(false, false)).toBe("offline");
  });
});
