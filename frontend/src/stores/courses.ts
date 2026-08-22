import { defineStore } from "pinia";
import { ref } from "vue";

import {
  byName,
  fetchPinnedCourses,
  fetchLiveStreams,
  fetchPublicCourses,
  fetchUserCourses,
  setCoursePinned,
  type Course,
  type LiveStream,
} from "@/lib/courses";
import { sameSemester, type Semester } from "@/lib/semesters";
import { useAuthStore } from "./auth";

/**
 * The course listings the start page is built from, held for one semester at a time.
 *
 * The sidebar and the view beside it show the same three listings, and the
 * server-rendered page fetched each of them twice for exactly that reason. Sharing
 * them here costs one request per listing per semester instead.
 *
 * Only the semester currently being looked at is kept. Caching every semester a user
 * visits would grow without bound for the sake of a back button that reloads in well
 * under a second.
 */
export const useCourseStore = defineStore("courses", () => {
  const publicCourses = ref<Course[]>([]);
  const userCourses = ref<Course[]>([]);
  /** Pins are not scoped to a semester, so these survive a semester change. */
  const pinnedCourses = ref<Course[]>([]);
  /** Live right now, so not scoped to a semester either. */
  const liveStreams = ref<LiveStream[]>([]);

  const loading = ref(false);
  const loaded = ref(false);
  /** The semester the listings above describe, so a repeat visit is free. */
  const semester = ref<Semester | undefined>(undefined);

  let pending: Promise<void> | null = null;

  /**
   * Loads the listings for a semester, or returns the ones already held for it.
   *
   * `undefined` means the current semester, which only the server knows — so it is
   * passed on as undefined rather than resolved here, and a later call naming the
   * same semester explicitly reloads once.
   */
  async function load(target: Semester | undefined, force = false): Promise<void> {
    const alreadyHave = loaded.value && sameSemester(semester.value, target);
    if (alreadyHave && !force) return;
    if (pending && !force) return pending;

    pending = fetchInto(target);
    try {
      await pending;
    } finally {
      pending = null;
    }
  }

  async function fetchInto(target: Semester | undefined): Promise<void> {
    loading.value = true;
    try {
      // The store shares one request however many components ask, so this is free
      // where the shell has already asked. Anonymous visitors skip the two listings
      // that would only answer them with a 401.
      const user = await useAuthStore().load();

      const [publicResult, userResult, pinnedResult, liveResult] = await Promise.all([
        fetchPublicCourses(target),
        user ? fetchUserCourses(target) : Promise.resolve([]),
        user ? fetchPinnedCourses() : Promise.resolve([]),
        fetchLiveStreams(),
      ]);

      publicCourses.value = publicResult;
      userCourses.value = userResult;
      pinnedCourses.value = pinnedResult;
      liveStreams.value = liveResult;
      semester.value = target;
      loaded.value = true;
    } finally {
      // Left unloaded on failure, so the next caller tries again.
      loading.value = false;
    }
  }

  /**
   * Pins or unpins a course, keeping every listing that holds it in step.
   *
   * The server is told first: a pin that failed to save must not be left showing in
   * the sidebar, where it would survive until the next reload and look saved.
   */
  async function togglePin(course: Course): Promise<void> {
    const pinned = !course.pinned;
    await setCoursePinned(course.id, pinned);

    for (const listing of [publicCourses, userCourses, pinnedCourses]) {
      for (const held of listing.value) {
        if (held.id === course.id) held.pinned = pinned;
      }
    }
    // The course being pinned may come from a listing this store does not hold — the
    // course page loads its own — so update the argument too.
    course.pinned = pinned;

    if (pinned) {
      if (!pinnedCourses.value.some((c) => c.id === course.id)) {
        pinnedCourses.value = [...pinnedCourses.value, course].sort(byName);
      }
    } else {
      pinnedCourses.value = pinnedCourses.value.filter((c) => c.id !== course.id);
    }
  }

  return {
    publicCourses,
    userCourses,
    pinnedCourses,
    liveStreams,
    loading,
    loaded,
    semester,
    load,
    togglePin,
  };
});
