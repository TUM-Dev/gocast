/**
 * How a user last chose to look at a course's lectures: ordering, grouping, whether
 * watched lectures are hidden, and grid or list.
 *
 * Kept in localStorage, as on the server-rendered page, but under `spa.` keys holding
 * words rather than the legacy numeric enums. Sharing the old keys would mean two
 * pages writing different encodings into one slot, and the values mean nothing outside
 * whichever page wrote them.
 */

import { ref, watch, type Ref } from "vue";

import type { GroupMode, SortMode } from "./lecture-groups";

export type ViewMode = "grid" | "list";

export interface LectureViewPrefs {
  sort: Ref<SortMode>;
  group: Ref<GroupMode>;
  hideWatched: Ref<boolean>;
  view: Ref<ViewMode>;
}

function stored<T extends string>(key: string, allowed: readonly T[], fallback: T): Ref<T> {
  // A value written by an older build, or by hand, is discarded rather than rendered.
  const saved = localStorage.getItem(key) as T | null;
  const state = ref(saved !== null && allowed.includes(saved) ? saved : fallback) as Ref<T>;

  watch(state, (value) => localStorage.setItem(key, value));
  return state;
}

export function useLectureViewPrefs(): LectureViewPrefs {
  const sort = stored<SortMode>("spa.lectureSort", ["newest", "oldest"], "newest");
  const group = stored<GroupMode>("spa.lectureGroup", ["month", "week"], "month");
  const view = stored<ViewMode>("spa.lectureView", ["grid", "list"], "grid");
  const hideWatched = stored<"yes" | "no">("spa.lectureHideWatched", ["yes", "no"], "no");

  return {
    sort,
    group,
    view,
    // Exposed as a boolean; stored as a word so the slot reads the same as the others.
    hideWatched: {
      get value() {
        return hideWatched.value === "yes";
      },
      set value(on: boolean) {
        hideWatched.value = on ? "yes" : "no";
      },
    } as Ref<boolean>,
  };
}
