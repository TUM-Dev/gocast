<script setup lang="ts">
import { Chart, type ChartConfiguration } from "chart.js/auto";
import { nextTick, onMounted, onUnmounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import { fetchServerStats, serverStatsExportLink, type ServerStats } from "@/lib/server-stats";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Server-wide usage statistics: the same charts and quick counters the old page drew
 * with Chart.js, now fed by getServerStats instead of api/statistics.go's
 * courseID == 0 case (which is unchanged and still serves the per-course page).
 */
const auth = useAuthStore();

const stats = ref<ServerStats | null>(null);
const loading = ref(true);
const error = ref("");

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to view server statistics.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

// One entry per canvas, in the order the old page laid them out. The label and color
// choices match api/statistics.go's chartJs responses exactly.
interface ChartDef {
  key: keyof Pick<ServerStats, "activityLive" | "activityVod" | "hourly" | "weekdays" | "allDays">;
  title: string;
  type: "line" | "bar";
  label: string;
  borderColor: string;
  backgroundColor: string;
}

const chartDefs: ChartDef[] = [
  {
    key: "activityLive",
    title: "Student Live activity per week",
    type: "line",
    label: "Live",
    borderColor: "#d12a5c",
    backgroundColor: "",
  },
  {
    key: "activityVod",
    title: "Student VoD activity per week",
    type: "line",
    label: "VoD",
    borderColor: "#2a7dd1",
    backgroundColor: "",
  },
  {
    key: "hourly",
    title: "VoD activity throughout the day",
    type: "bar",
    label: "Sum(viewers)",
    borderColor: "#427dbd",
    backgroundColor: "#427dbd",
  },
  {
    key: "weekdays",
    title: "VoD activity per day of week",
    type: "bar",
    label: "Sum(viewers)",
    borderColor: "#427dbd",
    backgroundColor: "#427dbd",
  },
  {
    key: "allDays",
    title: "VoD activity per day",
    type: "bar",
    label: "views",
    borderColor: "#d12a5c",
    backgroundColor: "#d12a5c",
  },
];

const canvases = ref<(HTMLCanvasElement | null)[]>([]);
let charts: Chart[] = [];

function destroyCharts(): void {
  charts.forEach((chart) => chart.destroy());
  charts = [];
}

function renderCharts(data: ServerStats): void {
  destroyCharts();

  chartDefs.forEach((def, index) => {
    const canvas = canvases.value[index];
    if (!canvas) return;

    const config: ChartConfiguration = {
      type: def.type,
      data: {
        labels: data[def.key].map((point) => point.x),
        datasets: [
          {
            label: def.label,
            data: data[def.key].map((point) => point.y),
            fill: false,
            borderColor: def.borderColor,
            backgroundColor: def.backgroundColor,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        scales: { y: { beginAtZero: true } },
      },
    };

    charts.push(new Chart(canvas, config));
  });
}

async function load(): Promise<void> {
  try {
    const data = await fetchServerStats();
    stats.value = data;
    error.value = "";
    // The canvases only exist once the v-else branch below renders.
    await nextTick();
    renderCharts(data);
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

onUnmounted(() => {
  destroyCharts();
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

      <div v-else-if="stats" class="grid grid-cols-1 gap-6 md:grid-cols-2">
        <table class="m-2 text-sm">
          <tbody class="text-3">
            <tr>
              <td class="pr-4">Enrolled Students</td>
              <td>{{ stats.numStudents }}</td>
            </tr>
            <tr>
              <td class="pr-4">Lectures</td>
              <td>{{ stats.numLectures }}</td>
            </tr>
            <tr>
              <td class="pr-4">Vod Views</td>
              <td>{{ stats.vodViews }}</td>
            </tr>
            <tr>
              <td class="pr-4">Live Views</td>
              <td>{{ stats.liveViews }}</td>
            </tr>
          </tbody>
        </table>

        <div v-for="(def, index) in chartDefs" :key="def.key">
          <h2 class="text-1 text-base font-semibold">{{ def.title }}</h2>
          <div class="m-auto w-full" style="min-height: 200px">
            <canvas
              :ref="(el) => (canvases[index] = el as HTMLCanvasElement | null)"
              :aria-label="def.title"
              role="img"
            ></canvas>
          </div>
        </div>

        <a
          :href="serverStatsExportLink('json')"
          class="tum-live-input-submit tum-live-button-muted block py-2 text-center text-sm"
          download="server-stats.json"
        >
          Export as JSON
        </a>
        <a
          :href="serverStatsExportLink('csv')"
          class="tum-live-input-submit tum-live-button-muted block py-2 text-center text-sm"
          download="server-stats.csv"
        >
          Export as CSV
        </a>

        <!--
          The old page only showed this for a course created in 2022 or earlier, or
          for courseID 0 -- which this page always is, so here it is unconditional.
        -->
        <p class="text-5 mx-auto block w-4/5 md:col-span-2">
          <i class="fas fa-info-circle text-warn"></i>
          Some of this data is only captured from June 28th 2021 onwards.
        </p>
      </div>
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
