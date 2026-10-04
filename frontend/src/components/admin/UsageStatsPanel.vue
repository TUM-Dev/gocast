<script setup lang="ts">
import { Chart, type ChartConfiguration } from "chart.js/auto";
import { onMounted, onUnmounted, ref, watch } from "vue";

import type { UsageStats } from "@/lib/usage-stats";

/**
 * The quick counters, charts and export links the server and course statistics
 * pages share. Mounted only once there is data, so the canvases exist when the
 * charts are drawn.
 */
const props = defineProps<{
  stats: UsageStats;
  /** Where each export format downloads from. */
  exportLinks: { json: string; csv: string };
  /** The downloaded file's name, without its extension. */
  exportName: string;
  /** Shows the note that viewing data only goes back to June 2021. */
  partialHistory: boolean;
}>();

// One entry per canvas, in the order the old pages laid them out. The label and color
// choices match api/statistics.go's chartJs responses exactly.
interface ChartDef {
  key: keyof Pick<UsageStats, "activityLive" | "activityVod" | "hourly" | "weekdays" | "allDays">;
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

function renderCharts(data: UsageStats): void {
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

onMounted(() => renderCharts(props.stats));
watch(
  () => props.stats,
  (stats) => renderCharts(stats),
);
onUnmounted(destroyCharts);
</script>

<template>
  <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
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
      :href="exportLinks.json"
      class="tum-live-input-submit tum-live-button-muted block py-2 text-center text-sm"
      :download="`${exportName}.json`"
    >
      Export as JSON
    </a>
    <a
      :href="exportLinks.csv"
      class="tum-live-input-submit tum-live-button-muted block py-2 text-center text-sm"
      :download="`${exportName}.csv`"
    >
      Export as CSV
    </a>

    <p v-if="partialHistory" class="text-5 mx-auto block w-4/5 md:col-span-2">
      <i class="fas fa-info-circle text-warn"></i>
      Some of this data is only captured from June 28th 2021 onwards.
    </p>
  </div>
</template>
