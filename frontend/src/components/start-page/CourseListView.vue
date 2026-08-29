<script setup lang="ts">
import CourseListItem from "@/components/start-page/CourseListItem.vue";
import type { Course } from "@/lib/courses";

/**
 * A full course listing with a way back to the start page. Ported from the
 * `#user-courses` and `#public-courses` sections of web/template/home.gohtml, which
 * were the same markup twice.
 */
defineProps<{ title: string; icon: string; courses: Course[] }>();
</script>

<template>
  <article class="tum-live-course-list h-full overflow-y-scroll">
    <header>
      <h1>
        <i class="text-base" :class="icon" />
        <span class="ml-2">{{ title }}</span>
      </h1>
      <RouterLink :to="{ name: 'home', query: $route.query }" title="Navigate to start">
        <i class="fa-solid fa-undo mr-2"></i>
        Go back
      </RouterLink>
    </header>

    <article class="grid gap-3 pb-4">
      <CourseListItem v-for="course in courses" :key="course.id" :course="course" />
    </article>
  </article>
</template>
