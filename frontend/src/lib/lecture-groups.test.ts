import { describe, expect, it } from "vitest";

import { groupLectures, sortLectures } from "./lecture-groups";
import type { Stream, StreamProgress } from "./streams";

const lecture = (id: number, iso: string): Stream => ({ id, start: new Date(iso) }) as Stream;

/** Mondays, one week apart, so the week boundaries are unambiguous. */
const week1 = lecture(1, "2026-10-12T10:00:00");
const week2 = lecture(2, "2026-10-19T10:00:00");
const week3 = lecture(3, "2026-10-26T10:00:00");

describe("sortLectures", () => {
  it("orders newest first by default and oldest first on request", () => {
    const lectures = [week2, week1, week3];

    expect(sortLectures(lectures, "newest").map((l) => l.id)).toEqual([3, 2, 1]);
    expect(sortLectures(lectures, "oldest").map((l) => l.id)).toEqual([1, 2, 3]);
  });

  it("does not reorder the array it was given", () => {
    const lectures = [week2, week1];
    sortLectures(lectures, "oldest");

    expect(lectures.map((l) => l.id)).toEqual([2, 1]);
  });
});

describe("grouping by month", () => {
  it("puts each month in its own group, named for it", () => {
    const november = lecture(4, "2026-11-02T10:00:00");
    const groups = groupLectures([week1, november], { group: "month", sort: "oldest" });

    expect(groups.map((g) => g.name)).toEqual(["October", "November"]);
    expect(groups[0].lectures.map((l) => l.id)).toEqual([1]);
  });

  it("orders the groups by the ordering asked for", () => {
    const november = lecture(4, "2026-11-02T10:00:00");
    const groups = groupLectures([week1, november], { group: "month", sort: "newest" });

    expect(groups.map((g) => g.name)).toEqual(["November", "October"]);
  });
});

describe("grouping by week", () => {
  it("numbers weeks from the first lecture", () => {
    const groups = groupLectures([week1, week2, week3], { group: "week", sort: "oldest" });

    expect(groups.map((g) => g.name)).toEqual(["Week 1", "Week 2", "Week 3"]);
  });

  /**
   * A semester has gaps — a public holiday, the Christmas break. Numbering by the
   * calendar would show a course jumping from Week 2 to Week 5; only the weeks that
   * have a lecture are counted.
   */
  it("skips weeks with no lecture rather than leaving a hole in the numbering", () => {
    const afterABreak = lecture(4, "2026-11-16T10:00:00"); // three weeks after week 3
    const groups = groupLectures([week1, week2, afterABreak], { group: "week", sort: "oldest" });

    expect(groups.map((g) => g.name)).toEqual(["Week 1", "Week 2", "Week 3"]);
  });

  /**
   * Weeks run from the Sunday before the first lecture, at 00:01 rather than midnight.
   * Without the offset a course whose first lecture is late on a Monday and whose
   * second week starts early would fold the two into one week.
   */
  it("keeps an earlier lecture in a later week out of the first one", () => {
    const lateMonday = lecture(1, "2026-10-12T18:00:00");
    const earlyMonday = lecture(2, "2026-10-19T08:00:00");
    const groups = groupLectures([lateMonday, earlyMonday], { group: "week", sort: "oldest" });

    expect(groups.map((g) => g.name)).toEqual(["Week 1", "Week 2"]);
  });

  it("groups two lectures in the same week together", () => {
    const alsoWeek1 = lecture(9, "2026-10-14T10:00:00");
    const groups = groupLectures([week1, alsoWeek1, week2], { group: "week", sort: "oldest" });

    expect(groups).toHaveLength(2);
    expect(groups[0].lectures.map((l) => l.id)).toEqual([1, 9]);
  });
});

describe("hiding watched lectures", () => {
  const progress = new Map<number, StreamProgress>([
    [1, { streamId: 1, progress: 1, watched: true }],
  ]);

  it("leaves out the ones already watched", () => {
    const groups = groupLectures([week1, week2], {
      group: "week",
      sort: "oldest",
      hideWatched: true,
      progress,
    });

    expect(groups.flatMap((g) => g.lectures).map((l) => l.id)).toEqual([2]);
  });

  /**
   * The numbering comes from every lecture, not the visible ones. Watching week one
   * and hiding it must not turn week two into "Week 1".
   */
  it("does not renumber the weeks that remain", () => {
    const groups = groupLectures([week1, week2], {
      group: "week",
      sort: "oldest",
      hideWatched: true,
      progress,
    });

    expect(groups.map((g) => g.name)).toEqual(["Week 2"]);
  });

  it("treats a lecture with no progress recorded as unwatched", () => {
    const groups = groupLectures([week2], {
      group: "week",
      sort: "oldest",
      hideWatched: true,
      progress: new Map(),
    });

    expect(groups).toHaveLength(1);
  });

  it("returns nothing when everything is hidden", () => {
    const groups = groupLectures([week1], {
      group: "week",
      sort: "oldest",
      hideWatched: true,
      progress,
    });

    expect(groups).toEqual([]);
  });
});

describe("an empty course", () => {
  it("has no groups rather than one empty one", () => {
    expect(groupLectures([], { group: "month", sort: "newest" })).toEqual([]);
  });
});
