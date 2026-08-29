<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";

import CourseListItem from "@/components/start-page/CourseListItem.vue";
import LiveStreamCard from "@/components/start-page/LiveStreamCard.vue";
import StartPageLayout from "@/components/start-page/StartPageLayout.vue";
import StreamThumbnail from "@/components/start-page/StreamThumbnail.vue";
import { courseUrl, type Course } from "@/lib/courses";
import { fetchServerNotifications, type ServerNotification } from "@/lib/notifications";
import {
  fetchProgress,
  isToday,
  minutesUntilStart,
  watchUrl,
  type StreamProgress,
} from "@/lib/streams";
import { useAuthStore } from "@/stores/auth";
import { useCourseStore } from "@/stores/courses";

/**
 * The start page proper, ported from the `#main-view` section of
 * web/template/home.gohtml: what is live, what is on today, the user's courses, and
 * the most recent recordings.
 */

/** Courses shown before the group hands off to the full listing, as on the old page. */
const USER_COURSES_SHOWN = 8;
/** Recent recordings revealed at a time, matching the old page's paginator. */
const RECENT_PAGE_SIZE = 10;

const auth = useAuthStore();
const courses = useCourseStore();

const serverNotifications = ref<ServerNotification[]>([]);
const progress = ref<Map<number, StreamProgress>>(new Map());
const recentShown = ref(RECENT_PAGE_SIZE);
const moreSentinel = ref<HTMLElement | null>(null);

const greeting = computed(() => {
  const user = auth.user;
  // The old page showed nothing at all for an account with no name.
  if (!user?.name) return null;
  return `${user.settings.greeting} ${user.settings.preferredName}, nice to see you! 👋`;
});

/** The user's courses whose next lecture is later today. */
const liveToday = computed(() =>
  courses.userCourses.filter(
    (course) =>
      course.nextLecture && isToday(course.nextLecture) && minutesUntilStart(course.nextLecture) > 0,
  ),
);

/**
 * Which courses the recent recordings are drawn from: the user's own, plus anything
 * they pinned that is not already among them, and the public listing for a visitor
 * with neither. Carried over from mainContext in web/ts/components/main.ts.
 */
const recentSource = computed<Course[]>(() => {
  if (courses.userCourses.length === 0) return courses.publicCourses;

  const own = new Set(courses.userCourses.map((c) => c.id));
  return [...courses.userCourses, ...courses.pinnedCourses.filter((c) => !own.has(c.id))];
});

const recent = computed(() => recentSource.value.filter((course) => course.lastRecording));
const recentVisible = computed(() => recent.value.slice(0, recentShown.value));
const hasMoreRecent = computed(() => recentShown.value < recent.value.length);

const isEmpty = computed(
  () =>
    courses.loaded &&
    courses.liveStreams.length === 0 &&
    recent.value.length === 0 &&
    courses.userCourses.length === 0 &&
    liveToday.value.length === 0,
);

function showMore(): void {
  recentShown.value += RECENT_PAGE_SIZE;
}

onMounted(() => {
  fetchServerNotifications()
    .then((notifications) => (serverNotifications.value = notifications))
    .catch(() => {});

  // The old page auto-loaded the next page as the end scrolled into view. The button
  // is rendered either way, so this only removes the need to press it.
  if (typeof IntersectionObserver !== "undefined") {
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting) && hasMoreRecent.value) showMore();
    });
    watch(moreSentinel, (element, _old, onCleanup) => {
      if (!element) return;
      observer.observe(element);
      onCleanup(() => observer.unobserve(element));
    });
  }
});

// Progress bars are the caller's own, so there is nothing to ask for when signed out.
watch(
  [recentVisible, () => auth.user],
  async ([visible, user]) => {
    if (!user) return;
    const missing = visible
      .map((course) => course.lastRecording!.id)
      .filter((id) => !progress.value.has(id));
    if (missing.length === 0) return;

    const loaded = await fetchProgress(missing).catch(() => new Map<number, StreamProgress>());
    progress.value = new Map([...progress.value, ...loaded]);
  },
  { immediate: true },
);

// A semester change replaces the listings, so the reveal starts over.
watch(
  () => courses.semester,
  () => (recentShown.value = RECENT_PAGE_SIZE),
);
</script>

<template>
  <StartPageLayout>
    <article class="relative min-h-full pb-8">
      <div
        v-for="(notification, index) in serverNotifications"
        :key="index"
        class="tum-live-notification mx-3 mb-3"
        :class="notification.warn ? 'tum-live-notification-warn' : 'tum-live-notification-info'"
      >
        <i
          class="icon fa-solid"
          :class="notification.warn ? 'fa-triangle-exclamation' : 'fa-circle-info'"
        ></i>
        <!-- Written by an administrator and rendered as markup, as on the old page. -->
        <!-- eslint-disable-next-line vue/no-v-html -->
        <span class="title" v-html="notification.html"></span>
      </div>

      <h1 v-if="greeting" id="greeting" class="mb-4 px-3 text-xl font-bold md:text-2xl">
        {{ greeting }}
      </h1>

      <span class="tum-live-content-grid">
        <article
          v-if="courses.liveStreams.length > 0"
          id="livestreams"
          class="tum-live-content-grid-item"
        >
          <h3 class="bg-danger mx-3 w-fit animate-pulse rounded py-1 text-sm uppercase text-white">
            Live
          </h3>
          <section class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
            <LiveStreamCard
              v-for="livestream in courses.liveStreams"
              :key="livestream.stream.id"
              :livestream="livestream"
            />
          </section>
        </article>

        <article v-if="liveToday.length > 0" id="live-today" class="tum-live-content-grid-item">
          <h3>Today</h3>
          <section class="grid gap-3 px-5">
            <article
              v-for="course in liveToday"
              :key="course.id"
              class="border-b px-1 py-2 last:border-0 dark:border-gray-800"
            >
              <RouterLink
                class="text-3 block font-semibold hover:cursor-pointer hover:underline"
                :to="courseUrl(course)"
              >
                {{ course.name }}
              </RouterLink>
              <div class="text-5 flex items-center text-sm font-light">
                <span>
                  Next lecture starts in {{ minutesUntilStart(course.nextLecture!) }} Minutes.
                  <a
                    :href="watchUrl(course.slug, course.nextLecture!.id)"
                    title="Join waiting room"
                  >
                    <i class="fa-solid fa-square-up-right"></i>
                    <span class="underline">Join waiting room</span>
                  </a>
                </span>
              </div>
            </article>
          </section>
        </article>

        <article
          v-if="courses.userCourses.length > 0"
          id="my-courses"
          class="tum-live-content-grid-item my-3"
        >
          <h3><i class="fa-solid fa-graduation-cap mr-2"></i>My Courses</h3>
          <article class="grid gap-2 py-3 pl-2">
            <CourseListItem
              v-for="course in courses.userCourses.slice(0, USER_COURSES_SHOWN)"
              :key="course.id"
              :course="course"
            />
          </article>
          <RouterLink
            v-if="courses.userCourses.length > USER_COURSES_SHOWN"
            :to="{ name: 'my-courses', query: $route.query }"
            class="tum-live-side-navigation-group-item hover ml-2"
          >
            <i class="fa-solid fa-chevron-right"></i>
            Show all my courses
          </RouterLink>
        </article>

        <article v-if="recent.length > 0" id="recent-vods" class="tum-live-content-grid-item">
          <h3>Recent VODs</h3>
          <section class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5">
            <article
              v-for="course in recentVisible"
              :key="course.id"
              class="tum-live-stream col-span-full p-3 sm:col-span-1"
            >
              <a :href="watchUrl(course.slug, course.lastRecording!.id)" class="mb-2 block">
                <StreamThumbnail
                  :stream="course.lastRecording!"
                  :slug="course.slug"
                  :progress="progress.get(course.lastRecording!.id)"
                />
              </a>
              <div class="px-1">
                <RouterLink
                  class="course"
                  :class="course.lastRecording!.name ? 'text-xs' : 'text-sm'"
                  :to="courseUrl(course)"
                >
                  {{ course.name }}
                </RouterLink>
                <a
                  v-if="course.lastRecording!.name"
                  class="title"
                  :href="watchUrl(course.slug, course.lastRecording!.id)"
                >
                  {{ course.lastRecording!.name }}
                </a>
                <span class="date text-xs">
                  {{ course.lastRecording!.start.toLocaleString("default", {
                    weekday: "long",
                    year: "numeric",
                    month: "numeric",
                    day: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  }) }}
                </span>
              </div>
            </article>
          </section>
          <button
            v-if="hasMoreRecent"
            ref="moreSentinel"
            type="button"
            class="tum-live-button tum-live-button-secondary mx-3 mt-3 w-fit"
            @click="showMore"
          >
            Show more
          </button>
        </article>

        <span v-if="isEmpty" class="m-auto font-semibold dark:text-white">
          No streams, VODs or courses to show in this semester.
        </span>
      </span>
    </article>
  </StartPageLayout>
</template>
