<script setup lang="ts">
import { onMounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import UsageStatsPanel from "@/components/admin/UsageStatsPanel.vue";
import { ApiError } from "@/lib/api";
import { fetchServerStats, serverStatsExportLink, type ServerStats } from "@/lib/server-stats";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Server-wide usage statistics: the same charts and quick counters the old page drew
 * with Chart.js, now fed by getServerStats instead of api/statistics.go's
 * courseID == 0 case.
 */
const auth = useAuthStore();

const stats = ref<ServerStats | null>(null);
const loading = ref(true);
const error = ref("");

const exportLinks = { json: serverStatsExportLink("json"), csv: serverStatsExportLink("csv") };

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to view server statistics.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function load(): Promise<void> {
  try {
    stats.value = await fetchServerStats();
    error.value = "";
  } catch (err) {
    error.value = message(err);
  } finally {
    loading.value = false;
  }
}

onMounted(async () => {
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
    return;
  }
  await load();
});
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-6">
      <h1 class="text-1 text-2xl font-bold">Server Statistics</h1>

      <Transition name="fade" mode="out-in">
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
          {{ error }}
        </p>
      </Transition>

      <p v-if="loading" class="text-5 text-sm">Loading statistics…</p>

      <!--
        The old page only showed the partial-history note for a course created in 2022
        or earlier, or for courseID 0 -- which this page always is.
      -->
      <UsageStatsPanel
        v-else-if="stats"
        :stats="stats"
        :export-links="exportLinks"
        export-name="server-stats"
        :partial-history="true"
      />
    </section>
  </AdminLayout>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
