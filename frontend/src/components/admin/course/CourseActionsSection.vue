<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";

import { ApiError } from "@/lib/api";
import { copyCourse, deleteCourse, type CourseAdmin } from "@/lib/course-admin";
import { fetchSemesters, semesterLabel, type TeachingTerm } from "@/lib/semesters";
import { courseError, useCourseAdminStore } from "@/stores/course-admin";

/** Copying the course into another semester, and deleting it. */
const props = defineProps<{ course: CourseAdmin }>();

const router = useRouter();
const store = useCourseAdminStore();

const copying = ref(false);
const year = ref(new Date().getFullYear());
const term = ref<TeachingTerm>("W");
const busy = ref(false);
const error = ref("");

// The tab stays mounted when a copy navigates to the new course; start that one closed.
watch(
  () => props.course.id,
  () => {
    copying.value = false;
    error.value = "";
  },
);

const target = computed(() => semesterLabel({ year: year.value, term: term.value }));
const validYear = computed(() => Number.isInteger(year.value) && year.value >= 1000 && year.value <= 9999);

onMounted(async () => {
  // Start from the semester the deployment considers current, as creating a course does.
  const semesters = await fetchSemesters().catch(() => null);
  if (semesters && !copying.value) {
    year.value = semesters.current.year;
    term.value = semesters.current.term;
  }
});

async function copy(): Promise<void> {
  busy.value = true;
  error.value = "";
  try {
    const copied = await copyCourse(props.course.id, { year: year.value, term: term.value });
    await store.loadAdministered(true);
    // The settings tab greets with this, and says how much did not come along.
    await router.push({
      path: `/admin/courses/${copied.courseId}/settings`,
      query: { copied: String(copied.numErrors) },
    });
  } catch (err) {
    error.value =
      err instanceof ApiError && err.status === 409
        ? `${target.value} already has a course with the slug "${props.course.slug}".`
        : courseError(err);
  } finally {
    busy.value = false;
  }
}

async function remove(): Promise<void> {
  const confirmed = window.confirm(
    `Delete "${props.course.name}" and all of its lectures? This cannot be undone.`,
  );
  if (!confirmed) return;
  busy.value = true;
  error.value = "";
  try {
    await deleteCourse(props.course.id);
    // The schedule is server data the SPA already owns; nothing there needs a reload.
    await store.loadAdministered(true);
    await router.push("/admin");
  } catch (err) {
    error.value = courseError(err);
    busy.value = false;
  }
}
</script>

<template>
  <section class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800" aria-labelledby="cs-actions">
    <h2 id="cs-actions" class="text-1 text-lg font-semibold">Actions</h2>

    <p
      v-if="error"
      class="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
      role="alert"
    >
      {{ error }}
    </p>

    <form v-if="copying" class="flex flex-col gap-3 text-sm" @submit.prevent="copy">
      <p class="text-3">
        Copies the settings, every lecture and the administrators into a new course in the semester you pick.
      </p>
      <div class="grid gap-4 sm:grid-cols-3">
        <div class="flex flex-col gap-1">
          <label for="cs-copy-term" class="text-2">Semester</label>
          <select id="cs-copy-term" v-model="term" class="tum-live-input">
            <option value="W">Wintersemester</option>
            <option value="S">Sommersemester</option>
          </select>
        </div>
        <div class="flex flex-col gap-1">
          <label for="cs-copy-year" class="text-2">Year</label>
          <input
            id="cs-copy-year"
            v-model.number="year"
            type="number"
            min="1000"
            max="9999"
            required
            class="tum-live-input"
          />
          <p class="text-5 text-xs">{{ target }}</p>
        </div>
      </div>
      <div class="flex flex-wrap justify-end gap-2">
        <button type="button" class="tum-live-button-secondary tum-live-button px-4 py-2 text-sm" :disabled="busy" @click="copying = false">
          Cancel
        </button>
        <button type="submit" class="tum-live-button-primary px-4 py-2 text-sm" :disabled="busy || !validYear">
          {{ busy ? "Copying…" : `Copy to ${target}` }}
        </button>
      </div>
    </form>

    <div v-else class="flex flex-col gap-2 sm:flex-row">
      <button type="button" class="tum-live-button-secondary tum-live-button px-4 py-2 text-sm" :disabled="busy" @click="copying = true">
        <i class="fa-regular fa-copy mr-2" aria-hidden="true"></i>Copy course and all its lectures
      </button>
      <button
        type="button"
        class="rounded-md bg-red-600 px-4 py-2 text-sm font-semibold text-white hover:bg-red-700 disabled:opacity-50 dark:bg-red-700 dark:hover:bg-red-600"
        :disabled="busy"
        @click="remove"
      >
        <i class="fa-regular fa-trash-can mr-2" aria-hidden="true"></i>Delete course and all its lectures
      </button>
    </div>
  </section>
</template>
