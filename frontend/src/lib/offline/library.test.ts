import { describe, expect, it } from "vitest";

import { clipOf, downloadFraction, expiryText, formatSize, groupByCourse, playableSeconds } from "./library";

describe("clipOf", () => {
  it("is the stored cut, or nothing", () => {
    expect(clipOf({})).toBeUndefined();
    expect(clipOf({ startOffset: 30 })).toEqual({ start: 30 });
    expect(clipOf({ startOffset: 30, endOffset: 600 })).toEqual({ start: 30, end: 600 });
    expect(clipOf({ startOffset: 30, endOffset: 10 })).toEqual({ start: 30 });
  });
});

describe("groupByCourse", () => {
  const l = (courseSlug: string, courseName: string, start: number) => ({ courseSlug, courseName, start });

  it("sorts courses by name and each course's lectures by date", () => {
    const groups = groupByCourse([l("eidi", "Informatik 1", 3), l("ana", "Analysis", 1), l("eidi", "Informatik 1", 2)]);
    expect(groups.map((g) => g.name)).toEqual(["Analysis", "Informatik 1"]);
    expect(groups[1]!.lectures.map((x) => x.start)).toEqual([2, 3]);
  });

  it("keeps two courses of the same name apart, and names a course without one", () => {
    const groups = groupByCourse([l("a", "Seminar", 1), l("b", "Seminar", 2), l("", "", 3)]);
    expect(groups.map((g) => g.key)).toEqual(["", "a", "b"]);
    expect(groups[0]!.name).toBe("Other lectures");
  });
});

describe("playableSeconds", () => {
  it("is the cut's length, or the recording's", () => {
    expect(playableSeconds({ durationSeconds: 3600 })).toBe(3600);
    expect(playableSeconds({ durationSeconds: 3600, startOffset: 600 })).toBe(3000);
    expect(playableSeconds({ durationSeconds: 3600, startOffset: 600, endOffset: 1800 })).toBe(1200);
    expect(playableSeconds({})).toBeUndefined();
  });
});

describe("downloadFraction", () => {
  const segments = [{ duration: 10, iv: "" }, { duration: 10, iv: "" }, { duration: 10, iv: "" }, { duration: 10, iv: "" }];
  it("prefers the live progress, else counts the stored segments", () => {
    expect(downloadFraction({ downloadedSegments: 1, segments }, 0.75)).toBe(0.75);
    expect(downloadFraction({ downloadedSegments: 1, segments }, undefined)).toBe(0.25);
    expect(downloadFraction({ downloadedSegments: 0, segments: [] }, undefined)).toBe(0);
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
