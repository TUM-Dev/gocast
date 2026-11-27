import { describe, expect, it } from "vitest";

import { advanceLastSeen, CLOCK_TOLERANCE_MS, isPlayable } from "./license";

const NOW = Date.UTC(2026, 9, 23, 12);
const DAY = 24 * 60 * 60 * 1000;

describe("isPlayable", () => {
  const saved = { state: "saved", expiresAt: NOW + 3 * DAY };

  it("plays a saved lecture within its licence", () => {
    expect(isPlayable(saved, NOW, NOW - DAY)).toBe(true);
  });

  it("refuses once the licence ran out", () => {
    expect(isPlayable(saved, saved.expiresAt, 0)).toBe(false);
  });

  it("refuses a lecture that is still being saved", () => {
    expect(isPlayable({ ...saved, state: "downloading" }, NOW, 0)).toBe(false);
  });

  it("refuses when the clock was turned back, but forgives a small correction", () => {
    const lastSeen = NOW + 2 * DAY;
    expect(isPlayable(saved, NOW, lastSeen)).toBe(false);
    expect(isPlayable(saved, lastSeen - CLOCK_TOLERANCE_MS + 1000, lastSeen)).toBe(true);
  });
});

describe("advanceLastSeen", () => {
  it("only moves forward", () => {
    expect(advanceLastSeen(NOW, NOW - DAY)).toBe(NOW);
    expect(advanceLastSeen(NOW, NOW + 1)).toBe(NOW + 1);
  });
});
