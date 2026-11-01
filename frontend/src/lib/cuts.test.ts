import { describe, expect, it } from "vitest";

import {
  MIN_CUT,
  acceptAll,
  acceptCut,
  addCut,
  appliedCuts,
  columnLevels,
  cutAt,
  formatTime,
  keptSeconds,
  leavesEnough,
  normalizeCuts,
  removeCut,
  resizeCut,
  suggestCuts,
  timeAfterCuts,
  timeAtX,
  toRanges,
  xAtTime,
  type Cut,
} from "./cuts";

const cut = (id: number, start: number, end: number, suggested = false): Cut => ({ id, start, end, suggested });
const spans = (cuts: Cut[]) => cuts.map((c) => [c.start, c.end]);

describe("normalizeCuts", () => {
  it("sorts, clamps and drops slivers", () => {
    const got = normalizeCuts([cut(1, 50, 60), cut(2, -5, 10), cut(3, 95, 140), cut(4, 30, 30.1)], 100);
    expect(spans(got)).toEqual([[0, 10], [50, 60], [95, 100]]);
  });

  it("joins overlapping and touching cuts", () => {
    expect(spans(normalizeCuts([cut(1, 10, 30), cut(2, 20, 40), cut(3, 40, 50)], 100))).toEqual([[10, 50]]);
  });

  it("does not join a suggestion to an accepted cut", () => {
    const got = normalizeCuts([cut(1, 10, 30), cut(2, 20, 40, true)], 100);
    expect(got).toHaveLength(2);
  });

  it("does not change its input", () => {
    const input = [cut(1, 50, 60), cut(2, 10, 20)];
    normalizeCuts(input, 100);
    expect(input[0].start).toBe(50);
  });
});

describe("addCut", () => {
  it("adds a cut dragged backwards", () => {
    expect(spans(addCut([], 40, 20, 100))).toEqual([[20, 40]]);
  });

  it("ignores a slip of the pointer", () => {
    expect(addCut([], 20, 20 + MIN_CUT / 2, 100)).toEqual([]);
  });

  it("joins the accepted cut it overlaps", () => {
    expect(spans(addCut([cut(1, 10, 30)], 25, 50, 100))).toEqual([[10, 50]]);
  });

  it("absorbs a suggestion it covers", () => {
    const got = addCut([cut(1, 20, 30, true)], 10, 40, 100);
    expect(spans(got)).toEqual([[10, 40]]);
    expect(got.every((c) => !c.suggested)).toBe(true);
  });

  it("leaves what it does not cover of a suggestion", () => {
    const got = addCut([cut(1, 10, 50, true)], 20, 30, 100);
    expect(got.map((c) => [c.start, c.end, c.suggested])).toEqual([
      [10, 20, true],
      [20, 30, false],
      [30, 50, true],
    ]);
    expect(new Set(got.map((c) => c.id)).size).toBe(3);
  });
});

describe("acceptCut and acceptAll", () => {
  it("applies one suggestion", () => {
    const got = acceptCut([cut(1, 10, 20, true), cut(2, 40, 50, true)], 1, 100);
    expect(appliedCuts(got).map((c) => [c.start, c.end])).toEqual([[10, 20]]);
    expect(got).toHaveLength(2);
  });

  it("applies all of them", () => {
    const got = acceptAll([cut(1, 10, 20, true), cut(2, 20, 30, true)], 100);
    expect(spans(got)).toEqual([[10, 30]]);
    expect(got[0].suggested).toBe(false);
  });

  it("answers an unknown id with the cuts as they were", () => {
    const cuts = [cut(1, 10, 20, true)];
    expect(acceptCut(cuts, 9, 100)).toEqual(cuts);
  });
});

describe("removeCut", () => {
  it("removes by id", () => {
    expect(spans(removeCut([cut(1, 1, 2), cut(2, 5, 6)], 1))).toEqual([[5, 6]]);
  });
});

describe("resizeCut", () => {
  const cuts = [cut(1, 10, 20), cut(2, 40, 50)];

  it("moves an edge", () => {
    expect(spans(resizeCut(cuts, 2, "start", 30, 100))).toEqual([[10, 20], [30, 50]]);
  });

  it("stops at the neighbour", () => {
    expect(spans(resizeCut(cuts, 2, "start", 5, 100))[1]).toEqual([20, 50]);
    expect(spans(resizeCut(cuts, 1, "end", 90, 100))[0]).toEqual([10, 40]);
  });

  it("keeps the cut at its least length", () => {
    expect(spans(resizeCut(cuts, 1, "start", 99, 100))[0]).toEqual([20 - MIN_CUT, 20]);
  });

  it("stops at the ends of the recording", () => {
    expect(spans(resizeCut(cuts, 2, "end", 500, 100))[1]).toEqual([40, 100]);
  });

  it("is not stopped by a suggestion", () => {
    const mixed = [cut(1, 10, 20, true), cut(2, 15, 50)];
    expect(spans(resizeCut(mixed, 2, "start", 0, 100))[1]).toEqual([0, 50]);
  });
});

describe("suggestCuts", () => {
  it("leaves padding at either edge of a silence", () => {
    const got = suggestCuts([{ start: 100, end: 200 }], 1000);
    expect(spans(got)).toEqual([[101.5, 198.5]]);
    expect(got[0].suggested).toBe(true);
  });

  it("does not pad the ends of the recording", () => {
    const got = suggestCuts(
      [
        { start: 0, end: 60 },
        { start: 900, end: 1000 },
      ],
      1000,
    );
    expect(spans(got)).toEqual([[0, 58.5], [901.5, 1000]]);
  });

  it("skips a silence too short to leave anything", () => {
    expect(suggestCuts([{ start: 100, end: 107 }], 1000)).toEqual([]);
  });

  it("gives each suggestion an id of its own", () => {
    const got = suggestCuts([{ start: 100, end: 200 }, { start: 300, end: 400 }], 1000);
    expect(new Set(got.map((c) => c.id)).size).toBe(2);
  });
});

describe("the result of the applied cuts", () => {
  const cuts = [cut(1, 10, 20), cut(2, 50, 60), cut(3, 70, 90, true)];

  it("counts only the accepted cuts", () => {
    expect(keptSeconds(cuts, 100)).toBe(80);
    expect(toRanges(cuts)).toEqual([{ start: 10, end: 20 }, { start: 50, end: 60 }]);
  });

  it("finds the cut a time lies in", () => {
    expect(cutAt(cuts, 15)?.id).toBe(1);
    expect(cutAt(cuts, 20)).toBeUndefined();
    expect(cutAt(cuts, 80)).toBeUndefined(); // a suggestion is not applied
  });

  it("maps a time onto the cut recording", () => {
    expect(timeAfterCuts(cuts, 5)).toBe(5);
    expect(timeAfterCuts(cuts, 15)).toBe(10);
    expect(timeAfterCuts(cuts, 100)).toBe(80);
  });

  it("wants a second left", () => {
    expect(leavesEnough([cut(1, 0, 99)], 100)).toBe(true);
    expect(leavesEnough([cut(1, 0, 99.5)], 100)).toBe(false);
  });
});

describe("the timeline", () => {
  it("maps pixels to time and back", () => {
    expect(timeAtX(250, 1000, 3600)).toBe(900);
    expect(xAtTime(900, 1000, 3600)).toBe(250);
    expect(timeAtX(-5, 1000, 3600)).toBe(0);
    expect(timeAtX(5000, 1000, 3600)).toBe(3600);
    expect(timeAtX(10, 0, 3600)).toBe(0);
    expect(xAtTime(10, 1000, 0)).toBe(0);
  });

  it("shows the loudest bucket of a column", () => {
    // 4 buckets of 1 s over 2 columns: 2 buckets each.
    expect(columnLevels([0, 255, 51, 102], 1, 4, 2)).toEqual([1, 0.4]);
  });

  it("stretches a short waveform over the columns", () => {
    const levels = columnLevels([255, 0], 1, 2, 4);
    expect(levels).toEqual([1, 1, 0, 0]);
  });

  it("is silent without data", () => {
    expect(columnLevels([], 1, 10, 3)).toEqual([0, 0, 0]);
    expect(columnLevels([255], 1, 0, 3)).toEqual([0, 0, 0]);
    expect(columnLevels([255], 1, 10, 0)).toEqual([]);
  });
});

describe("formatTime", () => {
  it("reads like the player's", () => {
    expect(formatTime(0)).toBe("0:00");
    expect(formatTime(242)).toBe("4:02");
    expect(formatTime(3842)).toBe("1:04:02");
    expect(formatTime(61.37, true)).toBe("1:01.3");
    expect(formatTime(-3)).toBe("0:00");
  });
});
