<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  createCourse,
  sanitizeSlug,
  searchTumOnlineCourses,
  type NewCourse,
  type TumOnlineCourse,
} from "@/lib/create-course";
import { fetchSemesters, semesterLabel } from "@/lib/semesters";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Creating a course: pick it from TUMOnline's course list, or type it in. Everything
 * beyond name, slug, semester and language is set afterwards on the course's page,
 * where the old form also sent its lecturers.
 */
const auth = useAuthStore();

const initialYear = new Date().getFullYear();

const form = reactive<NewCourse>({
  name: "",
  slug: "",
  year: initialYear,
  term: "W",
  tumOnlineId: "",
  language: "de",
});

const saving = ref(false);
const error = ref("");

const query = ref("");
const results = ref<TumOnlineCourse[]>([]);
const searchError = ref("");
const slugInput = ref<HTMLInputElement | null>(null);

const semester = computed(() => semesterLabel({ year: form.year, term: form.term }));
const complete = computed(
  () => form.name.trim() !== "" && form.slug !== "" && Number.isInteger(form.year) && form.year >= 1000,
);

onMounted(async () => {
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
    return;
  }
  // Default to the semester the deployment considers current rather than the old
  // form's hard-coded year; without it the form still works from today's year. Only
  // while the semester is untouched: the answer can arrive after someone has typed.
  const semesters = await fetchSemesters().catch(() => null);
  const untouched = form.year === initialYear && form.term === "W" && !form.tumOnlineId;
  if (semesters && untouched) {
    form.year = semesters.current.year;
    form.term = semesters.current.term;
  }
});

// The old form searched on every key press; this waits for a pause instead.
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let latestSearch = 0;
watch(query, (q) => {
  clearTimeout(searchTimer);
  if (q.trim() === "") {
    results.value = [];
    searchError.value = "";
    return;
  }
  searchTimer = setTimeout(async () => {
    const mine = ++latestSearch;
    try {
      const found = await searchTumOnlineCourses(q.trim());
      // A slower, older answer must not replace a newer one.
      if (mine !== latestSearch) return;
      results.value = found;
      searchError.value = "";
    } catch (err) {
      if (mine !== latestSearch) return;
      results.value = [];
      searchError.value =
        err instanceof ApiError && err.status === 503
          ? "The TUMOnline course search is unavailable right now. Enter the course manually below."
          : "The search failed. Enter the course manually below.";
    }
  }, 250);
});

function pick(course: TumOnlineCourse): void {
  form.tumOnlineId = course.tumOnlineId;
  form.name = course.name;
  form.year = course.year;
  form.term = course.term;
  query.value = "";
  results.value = [];
  // The slug is the one thing TUMOnline cannot suggest.
  slugInput.value?.focus();
}

function onSlugInput(event: Event): void {
  const input = event.target as HTMLInputElement;
  form.slug = sanitizeSlug(input.value);
  // Keep the field showing what was kept, not what was typed.
  input.value = form.slug;
}

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to create courses.";
    if (err.status === 409) return `${semester.value} already has a course with the slug "${form.slug}".`;
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function submit(): Promise<void> {
  error.value = "";
  saving.value = true;
  try {
    const id = await createCourse({ ...form, name: form.name.trim() });
    // The course's page is still server-rendered; ?created is what it greets with.
    window.location.assign(`/admin/course/${id}?created`);
  } catch (err) {
    error.value = message(err);
    saving.value = false;
  }
}
</script>

<template>
  <AdminLayout>
    <section class="mx-auto w-full max-w-4xl">
      <form
        class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800"
        @submit.prevent="submit"
      >
        <h1 class="text-1 text-2xl font-bold">Create Course</h1>

        <p
          v-if="error"
          class="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
          role="alert"
        >
          {{ error }}
        </p>

        <div class="flex flex-col gap-1 text-sm">
          <label for="cc-search" class="text-2">Find your course in TUMOnline</label>
          <input
            id="cc-search"
            v-model="query"
            type="search"
            autocomplete="off"
            placeholder="Search"
            class="tum-live-input"
            aria-controls="cc-results"
          />
          <p v-if="searchError" class="text-5 text-xs">{{ searchError }}</p>
          <ul v-if="results.length" id="cc-results" role="listbox" aria-label="TUMOnline courses">
            <li
              v-for="course in results"
              :key="course.tumOnlineId"
              role="option"
              aria-selected="false"
              tabindex="0"
              class="hover:bg-sky-500 dark:hover:bg-sky-600 m-2 cursor-pointer rounded-md bg-gray-200 p-2 dark:bg-gray-800"
              @click="pick(course)"
              @keydown.enter.prevent="pick(course)"
            >
              <p>{{ course.name }}</p>
              <p class="text-3 font-semibold">
                {{ semesterLabel({ year: course.year, term: course.term }) }}
              </p>
            </li>
          </ul>
        </div>

        <p class="text-4 text-center font-semibold">or enter it manually</p>

        <div class="grid gap-4 sm:grid-cols-[4fr_1fr]">
          <div class="flex flex-col gap-1 text-sm">
            <label for="cc-name" class="text-2"
              >Course title <span class="text-red-600 dark:text-red-400" aria-hidden="true">*</span></label
            >
            <input
              id="cc-name"
              v-model="form.name"
              type="text"
              autocomplete="off"
              placeholder="Einführung in die Informatik (IN0001)"
              required
              class="tum-live-input"
            />
          </div>
          <div class="flex flex-col gap-1 text-sm">
            <label for="cc-slug" class="text-2"
              >Slug <span class="text-red-600 dark:text-red-400" aria-hidden="true">*</span></label
            >
            <input
              id="cc-slug"
              ref="slugInput"
              :value="form.slug"
              type="text"
              autocomplete="off"
              placeholder="eidi"
              required
              class="tum-live-input"
              @input="onSlugInput"
            />
          </div>
        </div>
        <p class="text-5 -mt-2 text-xs">
          The slug is the course's short name in its address. Letters, digits, - and _ only.
        </p>

        <div class="grid gap-4 sm:grid-cols-3">
          <div class="flex flex-col gap-1 text-sm">
            <label for="cc-term" class="text-2">Semester</label>
            <select id="cc-term" v-model="form.term" class="tum-live-input">
              <option value="W">Wintersemester</option>
              <option value="S">Sommersemester</option>
            </select>
          </div>
          <div class="flex flex-col gap-1 text-sm">
            <label for="cc-year" class="text-2">Year</label>
            <input
              id="cc-year"
              v-model.number="form.year"
              type="number"
              min="1000"
              max="9999"
              required
              class="tum-live-input"
            />
            <p class="text-5 text-xs">{{ semester }}</p>
          </div>
          <div class="flex flex-col gap-1 text-sm">
            <label for="cc-lang" class="text-2">Language</label>
            <select id="cc-lang" v-model="form.language" class="tum-live-input">
              <option value="de">German</option>
              <option value="en">English</option>
              <option value="">-</option>
            </select>
          </div>
        </div>

        <p v-if="form.tumOnlineId" class="text-5 text-xs">
          Linked to TUMOnline course {{ form.tumOnlineId }}; its lectures and enrolments are
          fetched once the course exists.
        </p>

        <div class="flex items-center justify-end gap-2">
          <button
            type="submit"
            class="tum-live-button-primary px-4 py-2 text-sm"
            :disabled="saving || !complete"
          >
            {{ saving ? "Creating…" : "Create Course" }}
          </button>
        </div>
      </form>
    </section>
  </AdminLayout>
</template>
