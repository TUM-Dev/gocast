/**
 * Grouping and ordering for a course's recorded lectures.
 *
 * Kept as plain functions rather than component state because the week numbering has
 * a rule of its own, carried over from web/ts/components/course.ts, and it is the sort
 * of thing that goes wrong quietly: a course whose weeks are numbered from the wrong
 * starting Sunday looks perfectly plausible.
 */

import type { Stream, StreamProgress } from "./streams";

export type SortMode = "newest" | "oldest";
export type GroupMode = "month" | "week";

const MS_IN_WEEK = 1000 * 60 * 60 * 24 * 7;

const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

export interface LectureGroup {
  /** Rendered as the group's heading: a month name, or `Week 3`. */
  name: string;
  lectures: Stream[];
}

/**
 * The Sunday before the first lecture, which week numbers count from.
 *
 * The time is set to 00:01 rather than midnight so that a course whose first lecture is
 * at 10:00 and whose second week starts at 08:00 does not put the second one in week
 * one — carried over from initializeWeekMap.
 */
function startOfFirstWeek(lectures: Stream[]): Date {
  const earliest = lectures.reduce(
    (first, lecture) => (lecture.start < first.start ? lecture : first),
    lectures[0],
  );

  const sunday = new Date(earliest.start);
  sunday.setDate(sunday.getDate() - sunday.getDay());
  sunday.setHours(0, 1, 0, 0);
  return sunday;
}

function weekOf(lecture: Stream, firstWeek: Date): number {
  return Math.floor((lecture.start.getTime() - firstWeek.getTime()) / MS_IN_WEEK) + 1;
}

/**
 * Maps each calendar week that has a lecture to its position in the course.
 *
 * A semester has gaps — a public holiday, the Christmas break — and numbering by the
 * calendar would show a course jumping from Week 8 to Week 11. This numbers only the
 * weeks that actually have a lecture, so they read 1, 2, 3.
 */
function weekNumbering(lectures: Stream[], firstWeek: Date): Map<number, number> {
  const numbering = new Map<number, number>();
  let next = 1;

  for (const lecture of [...lectures].sort((a, b) => a.start.getTime() - b.start.getTime())) {
    const week = weekOf(lecture, firstWeek);
    if (!numbering.has(week)) numbering.set(week, next++);
  }

  return numbering;
}

export function sortLectures(lectures: Stream[], mode: SortMode): Stream[] {
  const ascending = [...lectures].sort((a, b) => a.start.getTime() - b.start.getTime());
  return mode === "oldest" ? ascending : ascending.reverse();
}

/**
 * Groups a course's lectures for display, in the requested order.
 *
 * `watched` is consulted only when hiding watched lectures; a lecture with no progress
 * recorded has not been watched.
 */
export function groupLectures(
  lectures: Stream[],
  options: {
    group: GroupMode;
    sort: SortMode;
    hideWatched?: boolean;
    progress?: Map<number, StreamProgress>;
  },
): LectureGroup[] {
  const visible = options.hideWatched
    ? lectures.filter((lecture) => !options.progress?.get(lecture.id)?.watched)
    : lectures;

  if (visible.length === 0) return [];

  // Numbered from every lecture, not only the visible ones: hiding the watched half of
  // a course must not renumber the weeks that remain.
  const firstWeek = startOfFirstWeek(lectures);
  const numbering = weekNumbering(lectures, firstWeek);

  const keyOf = (lecture: Stream): number =>
    options.group === "month"
      ? lecture.start.getMonth()
      : (numbering.get(weekOf(lecture, firstWeek)) ?? 0);

  const nameOf = (lecture: Stream): string =>
    options.group === "month"
      ? MONTHS[lecture.start.getMonth()]
      : `Week ${numbering.get(weekOf(lecture, firstWeek)) ?? 0}`;

  const groups = new Map<number, Stream[]>();
  for (const lecture of sortLectures(visible, options.sort)) {
    const key = keyOf(lecture);
    const existing = groups.get(key);
    if (existing) existing.push(lecture);
    else groups.set(key, [lecture]);
  }

  // Insertion order is the sorted order, so the groups come out in it too.
  return [...groups.values()].map((group) => ({ name: nameOf(group[0]), lectures: group }));
}
