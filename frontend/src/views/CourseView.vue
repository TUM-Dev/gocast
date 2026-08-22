<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRouter } from "vue-router";

import LiveStreamCard from "@/components/start-page/LiveStreamCard.vue";
import StartPageLayout from "@/components/start-page/StartPageLayout.vue";
import VodCard from "@/components/start-page/VodCard.vue";
import { ApiError } from "@/lib/api";
import { calendarUrl, fetchCourse, type Course } from "@/lib/courses";
import { groupLectures } from "@/lib/lecture-groups";
import { useLectureViewPrefs } from "@/lib/lecture-view-prefs";
import {
  fetchProgress,
  isMappedRoom,
  isToday,
  minutesUntilStart,
  roomUrl,
  setWatched,
  watchUrl,
  type Stream,
  type StreamProgress,
} from "@/lib/streams";
import { redirectToLogin, useAuthStore } from "@/stores/auth";
import { useCourseStore } from "@/stores/courses";

/**
 * One course: what is live, what is scheduled, and every recording. Ported from the
 * course view of web/template/home.gohtml, which reached it as `?view=3&slug=`.
 */
const props = defineProps<{ year: string; term: string; slug: string }>();

/** Scheduled lectures shown before the list offers the rest, as on the old page. */
const PLANNED_SHOWN = 3;

const auth = useAuthStore();
const store = useCourseStore();
const router = useRouter();
const prefs = useLectureViewPrefs();

const course = ref<Course | null>(null);
const progress = ref<Map<number, StreamProgress>>(new Map());
const showAllPlanned = ref(false);

/** Course administrators aside, a global admin reaches every course's admin page. */
const canAdminister = computed(() => course.value?.isAdmin || auth.user?.role === 1);

const recordings = computed(() => course.value?.streams.filter((s) => s.recording) ?? []);
const planned = computed(() =>
  [...(course.value?.streams.filter((s) => s.isPlanned) ?? [])].sort(
    (a, b) => a.start.getTime() - b.start.getTime(),
  ),
);
const upcoming = computed(() => course.value?.streams.filter((s) => s.isComingUp) ?? []);
const plannedVisible = computed(() =>
  showAllPlanned.value ? planned.value : planned.value.slice(0, PLANNED_SHOWN),
);

/** The lectures of this course that are live right now. */
const live = computed(() =>
  store.liveStreams.filter((entry) => entry.course.id === course.value?.id),
);

const groups = computed(() =>
  groupLectures(recordings.value, {
    group: prefs.group.value,
    sort: prefs.sort.value,
    hideWatched: prefs.hideWatched.value,
    progress: progress.value,
  }),
);

async function load(): Promise<void> {
  const year = Number(props.year);
  try {
    course.value = await fetchCourse(props.slug, {
      year: Number.isInteger(year) ? year : undefined,
      term: props.term === "S" ? "S" : "W",
    });
  } catch (err) {
    // A 401 means two different things here. To a visitor who is not signed in it
    // means the course might open once they are, so they are sent to sign in. To
    // someone already signed in it means they may not have this course at all —
    // sending them to a login page they have already been through explains nothing —
    // so they go back to the start page, as they do for a slug that no longer exists.
    const refused = err instanceof ApiError && err.isUnauthenticated;
    if (refused && !auth.user) redirectToLogin();
    else router.replace({ name: "home", query: { year: props.year, term: props.term } });
    return;
  }

  if (!auth.user || recordings.value.length === 0) return;
  progress.value = await fetchProgress(recordings.value.map((s) => s.id)).catch(
    () => new Map<number, StreamProgress>(),
  );
}

watch(
  () => [props.slug, props.year, props.term],
  () => {
    course.value = null;
    progress.value = new Map();
    showAllPlanned.value = false;
    load().catch(() => {});
  },
  { immediate: true },
);

async function toggleWatched(lecture: Stream): Promise<void> {
  const watched = !progress.value.get(lecture.id)?.watched;
  const updated = await setWatched(lecture.id, watched);
  progress.value = new Map([...progress.value, [lecture.id, updated]]);
}

function togglePin(): void {
  if (course.value) store.togglePin(course.value).catch(() => {});
}

const dayFormat = (date: Date, options: Intl.DateTimeFormatOptions) =>
  date.toLocaleString("default", options);

/** A scheduled lecture happening today is called out with a ribbon. */
const isTodayLecture = (lecture: Stream) => isToday(lecture);
</script>

<template>
  <StartPageLayout>
    <article v-if="course" class="tum-live-course-view">
      <header>
        <div>
          <p class="year">{{ course.semester.term }}{{ course.semester.year }}</p>
          <h1 class="name">{{ course.name }}</h1>
        </div>
        <section class="button-area">
          <button
            v-if="auth.user"
            type="button"
            class="tum-live-button tum-live-button-tertiary flex items-center text-xs"
            :class="{ active: course.pinned }"
            @click="togglePin"
          >
            <i class="fa-solid fa-thumbtack mr-2"></i>
            <span>{{ course.pinned ? "Pinned" : "Pin" }}</span>
          </button>

          <a
            :href="calendarUrl(course)"
            download
            class="tum-live-button tum-live-button-tertiary flex items-center text-xs"
            title="Download .ics calendar file"
          >
            <i class="fa-solid fa-calendar mr-2"></i>
            .ics
          </a>

          <a
            v-if="canAdminister"
            :href="`/admin/course/${course.id}`"
            class="tum-live-button tum-live-button-tertiary flex items-center text-xs"
            title="Go to admin page"
          >
            <i class="fa-solid fa-hammer mr-2"></i>
            Admin
          </a>
        </section>
      </header>

      <div class="grid grid-cols-1 gap-x-8 xl:grid-cols-3">
        <section v-if="live.length > 0" class="tum-live-course-view-item col-span-full">
          <!-- LiveStreamCard is a grid cell (`col-span-full lg:col-span-1`): without a
               grid around it the card fills the content width and its 16:9 thumbnail
               grows with it. Same columns as the start page's live row. -->
          <section class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3">
            <LiveStreamCard
              v-for="entry in live"
              :key="entry.stream.id"
              :livestream="entry"
            />
          </section>
        </section>

        <section v-if="upcoming.length > 0" class="tum-live-course-view-item col-span-full">
          <section class="tum-live-upcoming">
            <article
              v-for="lecture in upcoming"
              :key="lecture.id"
              class="tum-live-upcoming-item"
            >
              <span>Next lecture starts in {{ minutesUntilStart(lecture) }} Minutes.</span>
              <a :href="watchUrl(course.slug, lecture.id)" class="underline" title="Join waiting room">
                Join waiting room
              </a>
            </article>
          </section>
        </section>

        <section
          v-if="recordings.length > 0"
          class="tum-live-course-view-item"
          :class="planned.length > 0 ? 'col-span-2' : 'col-span-full'"
        >
          <header>
            <h3 class="font-bold">VODs</h3>
            <section class="button-area space-y-1">
              <button
                type="button"
                class="tum-live-button tum-live-button-tertiary"
                :class="{ active: prefs.sort.value === 'newest' }"
                @click="prefs.sort.value = 'newest'"
              >
                Newest first
              </button>
              <button
                type="button"
                class="tum-live-button tum-live-button-tertiary"
                :class="{ active: prefs.sort.value === 'oldest' }"
                @click="prefs.sort.value = 'oldest'"
              >
                Oldest first
              </button>
              <button
                v-if="auth.user"
                type="button"
                class="tum-live-button tum-live-button-tertiary"
                :class="{ active: prefs.hideWatched.value }"
                @click="prefs.hideWatched.value = !prefs.hideWatched.value"
              >
                Hide watched
              </button>
              <button
                type="button"
                class="tum-live-button tum-live-button-tertiary"
                :class="{ active: prefs.view.value === 'list' }"
                @click="prefs.view.value = prefs.view.value === 'list' ? 'grid' : 'list'"
              >
                List View
              </button>
              <button
                type="button"
                class="tum-live-button tum-live-button-tertiary"
                :class="{ active: prefs.group.value === 'week' }"
                @click="prefs.group.value = prefs.group.value === 'week' ? 'month' : 'week'"
              >
                Week View
              </button>
            </section>
          </header>

          <section>
            <article class="grid">
              <article v-for="group in groups" :key="group.name" class="mb-8">
                <header class="mb-2">
                  <h6 class="font-semibold">{{ group.name }}</h6>
                </header>
                <section
                  class="grid grid-cols-1 gap-3"
                  :class="
                    prefs.view.value === 'list'
                      ? 'md:grid-cols-1 lg:grid-cols-1 xl:grid-cols-1'
                      : planned.length > 0
                        ? 'md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'
                        : 'md:grid-cols-2 lg:grid-cols-4 xl:grid-cols-5'
                  "
                >
                  <VodCard
                    v-for="lecture in group.lectures"
                    :key="lecture.id"
                    :lecture="lecture"
                    :slug="course.slug"
                    :progress="progress.get(lecture.id)"
                    :signed-in="!!auth.user"
                    :downloads-enabled="course.downloadsEnabled"
                    :list-view="prefs.view.value === 'list'"
                    @toggle-watched="toggleWatched"
                  />
                </section>
              </article>
            </article>
          </section>
        </section>

        <section v-if="planned.length > 0" class="tum-live-course-view-item px-3">
          <header>
            <h3>Scheduled</h3>
          </header>
          <article>
            <section class="grid gap-5">
              <article
                v-for="lecture in plannedVisible"
                :key="lecture.id"
                class="rounded-lg"
                :class="{
                  'border dark:border-gray-800 dark:bg-gray-800': isTodayLecture(lecture),
                }"
              >
                <span
                  v-if="isTodayLecture(lecture)"
                  class="block w-full rounded-t-lg bg-red-500 px-2 py-px text-xs text-white"
                >
                  Today
                </span>
                <div class="flex items-center space-x-4 p-2">
                  <div class="w-11 text-center">
                    <p class="text-5 text-sm font-light">
                      {{ dayFormat(lecture.start, { month: "short" }) }}
                    </p>
                    <h1 class="text-3 font-bold">{{ lecture.start.getDate() }}</h1>
                  </div>
                  <div class="grow px-2">
                    <div class="flex items-center text-sm">
                      <i
                        class="tum-live-badge fa-solid fa-satellite-dish mr-2 bg-gray-100 px-2 text-xs dark:bg-gray-800"
                      ></i>
                      <span>
                        {{ dayFormat(lecture.start, { hour: "2-digit", minute: "2-digit" }) }} -
                        {{ dayFormat(lecture.end, { hour: "2-digit", minute: "2-digit" }) }}
                      </span>
                    </div>
                    <h4 class="text-1 font-semibold">{{ lecture.name }}</h4>
                    <a
                      v-if="isMappedRoom(lecture.roomCode)"
                      :href="roomUrl(lecture.roomCode)"
                      class="no-underline"
                      target="_blank"
                      rel="noopener"
                    >
                      {{ lecture.roomCode }}
                    </a>
                  </div>
                </div>
              </article>
            </section>
            <button
              v-if="!showAllPlanned && planned.length > PLANNED_SHOWN"
              type="button"
              class="tum-live-button tum-live-button-secondary mt-3 w-fit"
              @click="showAllPlanned = true"
            >
              Show all
            </button>
          </article>
        </section>
      </div>
    </article>
  </StartPageLayout>
</template>
