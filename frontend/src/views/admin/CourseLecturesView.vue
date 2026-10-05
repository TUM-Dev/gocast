<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";

import CourseLectureList from "@/components/admin/lectures/CourseLectureList.vue";
import { useCourseAdminStore } from "@/stores/course-admin";

/**
 * A course's lectures: the lectures tab of the course's administration page, which
 * supplies the frame, the course and the sign-in check. Everything on it is in
 * components/admin/lectures.
 *
 * The server has refused anyone who does not administer the course before serving
 * this page, and every RPC behind it refuses them again.
 */
const route = useRoute();
const store = useCourseAdminStore();

const courseId = computed(() => Number(route.params.courseID));
// The layout only renders its tabs once the course has loaded, so this is set.
const slug = computed(() => (store.course?.id === courseId.value ? store.course.slug : ""));
</script>

<template>
  <section class="flex flex-col gap-4">
    <h2 class="text-1 text-lg font-semibold">Lectures</h2>
    <CourseLectureList :course-id="courseId" :course-slug="slug" />
  </section>
</template>
