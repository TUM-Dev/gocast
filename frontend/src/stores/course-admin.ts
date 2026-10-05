import { defineStore } from "pinia";
import { ref } from "vue";

import { ApiError } from "@/lib/api";
import {
  fetchAdministeredCourses,
  fetchCourseAdmin,
  type AdministeredCourse,
  type CourseAdmin,
} from "@/lib/course-admin";
import { fetchSemesters, type Semester } from "@/lib/semesters";

/**
 * The course whose administration page is open, shared by its tabs, and the
 * sidebar's tree of every course the user administers.
 *
 * A store rather than provide/inject because both outlive a single component: the
 * tree is the same on every administration page, and fetching it again on each
 * navigation would make the sidebar flicker.
 */
export const useCourseAdminStore = defineStore("course-admin", () => {
  const course = ref<CourseAdmin | null>(null);
  const loading = ref(false);
  const error = ref("");
  /** Which course the latest load asked for, so a slower, older answer is dropped. */
  let latest = 0;

  async function load(courseId: number): Promise<void> {
    latest = courseId;
    if (course.value?.id !== courseId) {
      course.value = null;
    }
    loading.value = true;
    try {
      const loaded = await fetchCourseAdmin(courseId);
      if (latest !== courseId) return;
      course.value = loaded;
      error.value = "";
    } catch (err) {
      if (latest !== courseId) return;
      course.value = null;
      error.value = courseError(err);
    } finally {
      if (latest === courseId) loading.value = false;
    }
  }

  /** After a tab saved the course, so the header and the other tabs show it too. */
  function replace(updated: CourseAdmin): void {
    course.value = updated;
  }

  const administered = ref<AdministeredCourse[]>([]);
  const currentSemester = ref<Semester | null>(null);
  let treeLoaded = false;
  let treePending: Promise<void> | null = null;

  /**
   * Loads the sidebar's tree once per page load; `force` after a course was copied
   * or deleted. A failure leaves the tree empty rather than breaking the page it is
   * the sidebar of.
   */
  function loadAdministered(force = false): Promise<void> {
    if (treeLoaded && !force) return Promise.resolve();
    if (treePending && !force) return treePending;

    treePending = (async () => {
      const [courses, semesters] = await Promise.all([
        fetchAdministeredCourses().catch(() => null),
        currentSemester.value ? null : fetchSemesters().catch(() => null),
      ]);
      if (courses) {
        administered.value = courses;
        treeLoaded = true;
      }
      if (semesters) currentSemester.value = semesters.current;
    })().finally(() => {
      treePending = null;
    });
    return treePending;
  }

  return { course, loading, error, load, replace, administered, currentSemester, loadAdministered };
});

/** What the page says when a course-scoped call fails. */
export function courseError(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    // A course the caller does not administer answers the same as a missing one.
    if (err.status === 404) return "This course does not exist or you do not administer it.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}
