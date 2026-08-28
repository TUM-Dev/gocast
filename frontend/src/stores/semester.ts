import { defineStore } from "pinia";
import { computed, ref } from "vue";

import { fetchSemesters, sameSemester, type Semester, type TeachingTerm } from "@/lib/semesters";

/**
 * The semesters the deployment has, loaded once for the lifetime of the page.
 *
 * Which semester a page is *about* is not held here — that is in the URL, so a link
 * carries it and the back button works. The store only supplies the list the picker
 * renders and the current semester to fall back on.
 */
export const useSemesterStore = defineStore("semester", () => {
  const semesters = ref<Semester[]>([]);
  const current = ref<Semester | null>(null);
  const loading = ref(false);
  const loaded = ref(false);
  // The sidebar and the view it wraps both ask on mount; whoever asks while a request
  // is in flight waits for that one, as the auth store does.
  let pending: Promise<void> | null = null;

  async function load(force = false): Promise<void> {
    if (loaded.value && !force) return;
    if (pending && !force) return pending;

    pending = fetchInto();
    try {
      await pending;
    } finally {
      pending = null;
    }
  }

  async function fetchInto(): Promise<void> {
    loading.value = true;
    try {
      const res = await fetchSemesters();
      semesters.value = res.all;
      current.value = res.current;
      loaded.value = true;
    } finally {
      // Deliberately not marked loaded on failure, so the next caller tries again.
      loading.value = false;
    }
  }

  /**
   * The semester a page is about, given what its URL says.
   *
   * An explicit year and term win even when the list does not contain them — the list
   * is the semesters that have courses, and asking for an empty one should show it
   * empty rather than silently redirect. Undefined until the current semester is
   * known, which the API reads as "the current one" anyway, so a page can render
   * before the store has loaded.
   */
  function resolve(year?: number, term?: TeachingTerm): Semester | undefined {
    if (year !== undefined && term !== undefined) {
      return { year, term };
    }
    return current.value ?? undefined;
  }

  /** Whether a semester is one of the listed ones, for highlighting the picker. */
  const isKnown = computed(
    () => (semester: Semester | undefined) =>
      semesters.value.some((known) => sameSemester(known, semester)),
  );

  return { semesters, current, loading, loaded, load, resolve, isKnown };
});
