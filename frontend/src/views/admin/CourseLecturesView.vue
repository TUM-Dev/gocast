<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import CourseLectureList from "@/components/admin/lectures/CourseLectureList.vue";
import { fetchCourseHeader, type CourseHeader } from "@/lib/course-lectures";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * A course's lectures, the edit-course page's lecture tab. Stands alone in
 * AdminLayout for now; once the course administration layout exists this page's
 * route becomes one of its children and only CourseLectureList is mounted there.
 *
 * The server has refused anyone who does not administer the course before serving
 * this page, and every RPC behind it refuses them again.
 */
const auth = useAuthStore();
const route = useRoute();

const courseId = computed(() => Number(route.params.courseID));
const course = ref<CourseHeader | null>(null);
const ready = ref(false);

watch(
  courseId,
  async (id) => {
    const user = await auth.load().catch(() => null);
    if (!user) {
      redirectToLogin();
      return;
    }
    ready.value = true;
    course.value = null;
    // Only the heading and the stream keys need it; the list loads regardless.
    course.value = await fetchCourseHeader(id).catch(() => null);
  },
  { immediate: true },
);
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-4">
      <header>
        <h1 class="text-1 text-2xl font-bold">Lectures</h1>
        <p v-if="course" class="text-3">
          <!-- The course's settings are still a server-rendered page. -->
          <a :href="`/admin/course/${courseId}`" class="hover:underline">{{ course.name }}</a>
        </p>
      </header>
      <CourseLectureList v-if="ready" :course-id="courseId" :course-slug="course?.slug ?? ''" />
    </section>
  </AdminLayout>
</template>
