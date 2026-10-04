<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import UsageStatsPanel from "@/components/admin/UsageStatsPanel.vue";
import { ApiError } from "@/lib/api";
import { courseStatsExportLink, fetchCourseStats, type CourseStats } from "@/lib/course-stats";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * One course's usage statistics, for its lecturers. The server has already refused
 * anyone who does not administer the course before serving this page; getCourseStats
 * refuses them again, so the page cannot be the only guard.
 */
const auth = useAuthStore();
const route = useRoute();

const courseId = computed(() => Number(route.params.courseID));

const stats = ref<CourseStats | null>(null);
const loading = ref(true);
const error = ref("");

const exportLinks = computed(() => ({
  json: courseStatsExportLink(courseId.value, "json"),
  csv: courseStatsExportLink(courseId.value, "csv"),
}));

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    // A course the caller does not administer answers the same as a missing one.
    if (err.status === 404) return "This course does not exist or you do not administer it.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function load(id: number): Promise<void> {
  loading.value = true;
  stats.value = null;
  try {
    stats.value = await fetchCourseStats(id);
    error.value = "";
  } catch (err) {
    error.value = message(err);
  } finally {
    loading.value = false;
  }
}

watch(
  courseId,
  async (id) => {
    const user = await auth.load().catch(() => null);
    if (!user) {
      redirectToLogin();
      return;
    }
    await load(id);
  },
  { immediate: true },
);
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-6">
      <header>
        <h1 class="text-1 text-2xl font-bold">Statistics</h1>
        <p v-if="stats" class="text-3">
          <!-- The course's settings are still a server-rendered page. -->
          <a :href="`/admin/course/${courseId}`" class="hover:underline">{{ stats.courseName }}</a>
        </p>
      </header>

      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
        {{ error }}
      </p>

      <p v-if="loading" class="text-5 text-sm">Loading statistics…</p>

      <UsageStatsPanel
        v-else-if="stats"
        :stats="stats"
        :export-links="exportLinks"
        :export-name="`course-${courseId}-stats`"
        :partial-history="stats.partialHistory"
      />
    </section>
  </AdminLayout>
</template>
