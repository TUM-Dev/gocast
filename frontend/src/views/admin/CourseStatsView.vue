<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";

import StatsPanel from "@/components/admin/StatsPanel.vue";
import { ApiError } from "@/lib/api";
import { courseStatsExportLink, fetchCourseStats, type CourseStats } from "@/lib/course-stats";
import { usageCharts, usageCounters } from "@/lib/usage-stats";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * One course's usage statistics, for its lecturers: the statistics tab of the course's
 * administration page, which supplies the frame and the course's name. The server has
 * already refused anyone who does not administer the course before serving this page;
 * getCourseStats refuses them again, so the page cannot be the only guard.
 */
const auth = useAuthStore();
const route = useRoute();

const courseId = computed(() => Number(route.params.courseID));

const stats = ref<CourseStats | null>(null);
const loading = ref(true);

// Computed rather than called in the template: a fresh array on every render would
// make StatsPanel redraw its charts each time.
const counters = computed(() => (stats.value ? usageCounters(stats.value) : []));
const charts = computed(() => (stats.value ? usageCharts(stats.value) : []));
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
  <section class="flex flex-col gap-6">
    <h2 class="text-1 text-lg font-semibold">Statistics</h2>

    <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
      {{ error }}
    </p>

    <p v-if="loading" class="text-5 text-sm">Loading statistics…</p>

    <StatsPanel
      v-else-if="stats"
      :counters="counters"
      :charts="charts"
      :export-links="exportLinks"
      :export-name="`course-${courseId}-stats`"
      :partial-history="stats.partialHistory"
    />
  </section>
</template>
