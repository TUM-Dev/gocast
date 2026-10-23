import { describe, expect, it } from "vitest";

import { clipOf, expiryText, formatSize } from "./library";

describe("clipOf", () => {
  it("is the stored cut, or nothing", () => {
    expect(clipOf({})).toBeUndefined();
    expect(clipOf({ startOffset: 30 })).toEqual({ start: 30 });
    expect(clipOf({ startOffset: 30, endOffset: 600 })).toEqual({ start: 30, end: 600 });
    expect(clipOf({ startOffset: 30, endOffset: 10 })).toEqual({ start: 30 });
  });
});

describe("formatSize", () => {
  it("rounds to a readable unit", () => {
    expect(formatSize(1_234_000_000)).toBe("1.2 GB");
    expect(formatSize(350_400_000)).toBe("350 MB");
    expect(formatSize(12)).toBe("1 kB");
  });
});

describe("expiryText", () => {
  const now = Date.UTC(2026, 9, 23);
  it("counts days, then hours, then says it expired", () => {
    expect(expiryText(now + 6.5 * 86_400_000, now)).toBe("Plays offline for 6 more days");
    expect(expiryText(now + 86_400_000, now)).toBe("Plays offline for 1 more day");
    expect(expiryText(now + 3 * 3_600_000, now)).toBe("Plays offline for 3 more hours");
    expect(expiryText(now, now)).toContain("Expired");
  });
});
