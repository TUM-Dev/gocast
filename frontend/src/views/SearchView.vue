<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";

import type { Course } from "@/lib/courses";
import { singleQueryParam } from "@/lib/route-query";
import {
  MAX_FILTER_COURSES,
  MIN_QUERY_LENGTH,
  coursesOfSemesters,
  courseHitUrl,
  courseKey,
  formatTimestamp,
  kindsFor,
  search,
  searchErrorMessage,
  semesterKey,
  streamHitUrl,
  subtitleHitUrl,
  type SearchFilters,
  type SearchHits,
} from "@/lib/search";
import { fetchSemesters, semesterLabel, type Semester } from "@/lib/semesters";
import { useAuthStore } from "@/stores/auth";

/**
 * The search page. The query lives in `?q=` so a search can be linked and the header
 * field can land here; semesters narrow it, and picking courses of those semesters
 * searches their lectures and subtitle lines instead, as the old page did.
 */
const route = useRoute();
const router = useRouter();
const auth = useAuthStore();

const query = ref(singleQueryParam(route.query, "q") ?? "");
const semesters = ref<Semester[]>([]);
const chosenSemesters = ref<Semester[]>([]);
const courses = ref<Course[]>([]);
const chosenCourses = ref<Course[]>([]);
const courseFilter = ref("");
const semestersOpen = ref(false);
const coursesOpen = ref(false);

const hits = ref<SearchHits | null>(null);
const searching = ref(false);
const error = ref("");

const filters = computed<SearchFilters>(() => ({ semesters: chosenSemesters.value, courses: chosenCourses.value }));
const kinds = computed(() => kindsFor(filters.value));
const shownCourses = computed(() => {
  const needle = courseFilter.value.trim().toLowerCase();
  return needle ? courses.value.filter((c) => c.name.toLowerCase().includes(needle)) : courses.value;
});
const tooShort = computed(() => query.value.trim().length < MIN_QUERY_LENGTH);

const isChosenSemester = (s: Semester) => chosenSemesters.value.some((c) => semesterKey(c) === semesterKey(s));
const isChosenCourse = (c: Course) => chosenCourses.value.some((x) => courseKey(x) === courseKey(c));

function toggleSemester(s: Semester, on: boolean): void {
  chosenSemesters.value = on
    ? [...chosenSemesters.value, s]
    : chosenSemesters.value.filter((c) => semesterKey(c) !== semesterKey(s));
}

function toggleCourse(c: Course, on: boolean): void {
  if (on && chosenCourses.value.length >= MAX_FILTER_COURSES) return;
  chosenCourses.value = on ? [...chosenCourses.value, c] : chosenCourses.value.filter((x) => courseKey(x) !== courseKey(c));
}

let timer: ReturnType<typeof setTimeout> | undefined;
let latest = 0;

async function run(): Promise<void> {
  const q = query.value.trim();
  if (q.length < MIN_QUERY_LENGTH) {
    hits.value = null;
    error.value = "";
    return;
  }
  const id = ++latest;
  searching.value = true;
  try {
    const found = await search(q, filters.value);
    if (id !== latest) return;
    hits.value = found;
    error.value = "";
  } catch (err) {
    if (id !== latest) return;
    hits.value = null;
    error.value = searchErrorMessage(err);
  } finally {
    if (id === latest) searching.value = false;
  }
}

function schedule(): void {
  clearTimeout(timer);
  timer = setTimeout(run, 300);
}

watch(query, (q) => {
  void router.replace({ query: { ...route.query, q: q.trim() || undefined } });
  schedule();
});

watch(chosenSemesters, async (chosen) => {
  // The course filter only offers courses of the chosen semesters.
  chosenCourses.value = chosenCourses.value.filter((c) => chosen.some((s) => semesterKey(s) === semesterKey(c.semester)));
  courses.value = chosen.length ? await coursesOfSemesters(chosen, auth.user !== null).catch(() => []) : [];
  schedule();
});
watch(chosenCourses, schedule);

onMounted(async () => {
  await auth.load().catch(() => null);
  fetchSemesters()
    .then((s) => (semesters.value = s.all))
    .catch(() => (semesters.value = []));
  void run();
});
</script>

<template>
  <section class="text-3 mx-auto flex w-full max-w-4xl flex-col gap-4 p-4">
    <header class="flex flex-col gap-3">
      <h1 class="text-1 text-2xl font-bold">Search</h1>
      <label class="flex flex-col gap-1 text-sm">
        <span class="text-5">Search for courses, lectures and subtitles</span>
        <input
          v-model="query"
          type="search"
          autofocus
          autocomplete="off"
          placeholder="Search for course"
          class="tum-live-input w-full"
        />
      </label>

      <div class="flex flex-wrap gap-2 text-sm">
        <div class="relative">
          <button
            type="button"
            class="tum-live-button-secondary tum-live-button px-3 py-1"
            :aria-expanded="semestersOpen"
            aria-controls="search-semesters"
            @click="semestersOpen = !semestersOpen; coursesOpen = false"
          >
            Semester<span v-if="chosenSemesters.length"> ({{ chosenSemesters.length }})</span>
          </button>
          <fieldset
            v-show="semestersOpen"
            id="search-semesters"
            class="absolute left-0 z-20 mt-2 max-h-80 w-56 overflow-y-auto rounded-lg border bg-white p-3 shadow dark:border-gray-700 dark:bg-gray-800"
          >
            <legend class="sr-only">Semesters to search in</legend>
            <p v-if="!semesters.length" class="text-5">No semesters.</p>
            <label v-for="s in semesters" :key="semesterKey(s)" class="block">
              <input
                type="checkbox"
                :checked="isChosenSemester(s)"
                @change="toggleSemester(s, ($event.target as HTMLInputElement).checked)"
              />
              {{ semesterLabel(s) }}
            </label>
          </fieldset>
        </div>

        <div v-if="chosenSemesters.length" class="relative">
          <button
            type="button"
            class="tum-live-button-secondary tum-live-button px-3 py-1"
            :aria-expanded="coursesOpen"
            aria-controls="search-courses"
            @click="coursesOpen = !coursesOpen; semestersOpen = false"
          >
            Courses<span v-if="chosenCourses.length"> ({{ chosenCourses.length }})</span>
          </button>
          <fieldset
            v-show="coursesOpen"
            id="search-courses"
            class="absolute left-0 z-20 mt-2 max-h-80 w-72 overflow-y-auto rounded-lg border bg-white p-3 shadow dark:border-gray-700 dark:bg-gray-800"
          >
            <legend class="sr-only">Courses to search in</legend>
            <input v-model="courseFilter" type="search" placeholder="Filter courses" class="tum-live-input mb-2 w-full" />
            <p class="text-5 mb-1 text-xs">Up to {{ MAX_FILTER_COURSES }}. Picking courses searches their lectures and subtitles.</p>
            <p v-if="!shownCourses.length" class="text-5">No course found.</p>
            <label v-for="c in shownCourses" :key="courseKey(c)" class="block">
              <input
                type="checkbox"
                :checked="isChosenCourse(c)"
                :disabled="!isChosenCourse(c) && chosenCourses.length >= MAX_FILTER_COURSES"
                @change="toggleCourse(c, ($event.target as HTMLInputElement).checked)"
              />
              {{ c.name }} <span class="text-5">{{ semesterLabel(c.semester) }}</span>
            </label>
          </fieldset>
        </div>
      </div>
    </header>

    <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>
    <p v-else-if="tooShort" class="text-5 text-sm">Type at least {{ MIN_QUERY_LENGTH }} characters to search.</p>
    <p v-else-if="searching && !hits" class="text-5 text-sm" role="status">Searching…</p>

    <template v-if="hits">
      <section aria-labelledby="search-courses-heading">
        <h2 id="search-courses-heading" class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700">
          Courses
        </h2>
        <p v-if="!hits.courses.length" class="text-5 py-2 text-sm">No course found.</p>
        <ul v-else class="divide-y dark:divide-gray-700">
          <li v-for="hit in hits.courses" :key="courseKey(hit)">
            <RouterLink :to="courseHitUrl(hit)" class="hover:text-1 block py-2">
              <span class="text-2 font-medium">{{ hit.name }}</span>
              <span class="text-5 ml-2 text-sm">{{ semesterLabel(hit.semester) }}</span>
            </RouterLink>
          </li>
        </ul>
      </section>

      <section aria-labelledby="search-streams-heading">
        <h2 id="search-streams-heading" class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700">
          Lectures
        </h2>
        <p v-if="!kinds.streams" class="text-5 py-2 text-sm">Pick one semester, or some courses, to search lectures.</p>
        <p v-else-if="!hits.streams.length" class="text-5 py-2 text-sm">No lecture found.</p>
        <ul v-else class="divide-y dark:divide-gray-700">
          <li v-for="hit in hits.streams" :key="hit.id">
            <!-- The watch page is still server-rendered. -->
            <a :href="streamHitUrl(hit)" class="hover:text-1 block py-2">
              <span class="text-2 font-medium">{{ hit.name }}</span>
              <span class="text-5 ml-2 text-sm">{{ hit.courseName }} · {{ semesterLabel(hit.semester) }}</span>
            </a>
          </li>
        </ul>
      </section>

      <section aria-labelledby="search-subtitles-heading">
        <h2 id="search-subtitles-heading" class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700">
          Subtitles
        </h2>
        <p v-if="!kinds.subtitles" class="text-5 py-2 text-sm">Pick some courses to search their subtitles.</p>
        <p v-else-if="!hits.subtitles.length" class="text-5 py-2 text-sm">No subtitle found.</p>
        <ul v-else class="divide-y dark:divide-gray-700">
          <li v-for="hit in hits.subtitles" :key="`${hit.streamId}-${hit.timestamp}`">
            <a :href="subtitleHitUrl(hit)" class="hover:text-1 block py-2">
              <p>
                <span class="text-5">{{ hit.textPrev }} </span><span class="text-1">{{ hit.text }}</span
                ><span class="text-5"> {{ hit.textNext }}</span>
              </p>
              <p class="text-5 text-sm">
                {{ hit.courseName }} · {{ hit.streamName || "Lecture" }} · {{ formatTimestamp(hit.timestamp) }}
              </p>
            </a>
          </li>
        </ul>
      </section>
    </template>
  </section>
</template>
