<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import StatsPanel from "@/components/admin/StatsPanel.vue";
import { ApiError } from "@/lib/api";
import {
  fetchLectureStats,
  lectureCharts,
  lectureCounters,
  type LectureStats,
} from "@/lib/lecture-stats";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * One lecture's usage statistics. As on the course page, the server refuses anyone
 * who does not administer the course before serving this page, and getLectureStats
 * refuses them again -- and also refuses a lecture that is not the course's.
 */
const auth = useAuthStore();
const route = useRoute();

const courseId = computed(() => Number(route.params.courseID));
const streamId = computed(() => Number(route.params.streamID));

const stats = ref<LectureStats | null>(null);
const loading = ref(true);
const error = ref("");

// Computed rather than called in the template: a fresh array on every render would
// make StatsPanel redraw its charts each time.
const counters = computed(() => (stats.value ? lectureCounters(stats.value) : []));
const charts = computed(() => (stats.value ? lectureCharts(stats.value) : []));

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    // A lecture outside the course answers the same as a missing one.
    if (err.status === 404) return "This lecture does not exist or you do not administer it.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function load(course: number, stream: number): Promise<void> {
  loading.value = true;
  stats.value = null;
  try {
    stats.value = await fetchLectureStats(course, stream);
    error.value = "";
  } catch (err) {
    error.value = message(err);
  } finally {
    loading.value = false;
  }
}

watch(
  [courseId, streamId],
  async ([course, stream]) => {
    const user = await auth.load().catch(() => null);
    if (!user) {
      redirectToLogin();
      return;
    }
    await load(course, stream);
  },
  { immediate: true },
);
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-6">
      <header>
        <h1 class="text-1 text-2xl font-bold">Lecture Statistics</h1>
        <p v-if="stats" class="text-3">
          <RouterLink :to="`/admin/courses/${courseId}/stats`" class="hover:underline">{{
            stats.courseName
          }}</RouterLink>
          <template v-if="stats.lectureName"> · {{ stats.lectureName }}</template>
        </p>
      </header>

      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
        {{ error }}
      </p>

      <p v-if="loading" class="text-5 text-sm">Loading statistics…</p>

      <StatsPanel
        v-else-if="stats"
        :counters="counters"
        :charts="charts"
        :partial-history="stats.partialHistory"
      />
    </section>
  </AdminLayout>
</template>
