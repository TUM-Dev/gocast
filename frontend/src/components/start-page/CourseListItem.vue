<script setup lang="ts">
import { courseUrl, type Course } from "@/lib/courses";
import { watchUrl } from "@/lib/streams";

/**
 * One course in a list: its name, when the next lecture is, and a link to the most
 * recent recording. Ported from the `tum-live-course-list-item` markup repeated four
 * times in web/template/home.gohtml.
 */
defineProps<{ course: Course }>();

/** Matches Stream.FriendlyDateStart in web/ts/api/courses.ts. */
function friendlyDate(date: Date): string {
  return date.toLocaleString("default", {
    weekday: "long",
    year: "numeric",
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
</script>

<template>
  <section class="tum-live-course-list-item">
    <RouterLink class="title" :to="courseUrl(course)">{{ course.name }}</RouterLink>
    <div class="links">
      <span v-if="course.nextLecture">
        Next lecture: {{ friendlyDate(course.nextLecture.start) }}
      </span>
      <span v-else>No upcoming lecture.</span>

      <!-- Full navigation: the watch page is still server-rendered. -->
      <a v-if="course.lastRecording" :href="watchUrl(course.slug, course.lastRecording.id)">
        <i class="fa-solid fa-square-up-right"></i>
        <span class="hover:underline">
          Most recent lecture: {{ friendlyDate(course.lastRecording.start) }}
        </span>
      </a>
    </div>
  </section>
</template>
