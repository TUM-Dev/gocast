<script setup lang="ts">
import { Chart, type ChartConfiguration } from "chart.js/auto";
import { onMounted, onUnmounted, ref, watch } from "vue";

import type { StatsChart, StatsCounter } from "@/lib/usage-stats";

/**
 * The quick counters, charts and export links every statistics page shows. Mounted
 * only once there is data, so the canvases exist when the charts are drawn.
 */
const props = defineProps<{
  counters: StatsCounter[];
  charts: StatsChart[];
  /** Where each export format downloads from; the links are left out without it. */
  exportLinks?: { json: string; csv: string };
  /** The downloaded file's name, without its extension. */
  exportName?: string;
  /** Shows the note that viewing data only goes back to June 2021. */
  partialHistory: boolean;
}>();

const canvases = ref<(HTMLCanvasElement | null)[]>([]);
let drawn: Chart[] = [];

function destroyCharts(): void {
  drawn.forEach((chart) => chart.destroy());
  drawn = [];
}

function renderCharts(charts: StatsChart[]): void {
  destroyCharts();

  charts.forEach((def, index) => {
    const canvas = canvases.value[index];
    if (!canvas) return;

    const config: ChartConfiguration = {
      type: def.type,
      data: {
        labels: def.points.map((point) => point.x),
        datasets: [
          {
            label: def.label,
            data: def.points.map((point) => point.y),
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

    drawn.push(new Chart(canvas, config));
  });
}

onMounted(() => renderCharts(props.charts));
watch(
  () => props.charts,
  (charts) => renderCharts(charts),
);
onUnmounted(destroyCharts);
</script>

<template>
  <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
    <table class="m-2 text-sm">
      <tbody class="text-3">
        <tr v-for="counter in counters" :key="counter.label">
          <td class="pr-4">{{ counter.label }}</td>
          <td>{{ counter.value }}</td>
        </tr>
      </tbody>
    </table>

    <div v-for="(def, index) in charts" :key="def.title">
      <h2 class="text-1 text-base font-semibold">{{ def.title }}</h2>
      <div class="m-auto w-full" style="min-height: 200px">
        <canvas
          :ref="(el) => (canvases[index] = el as HTMLCanvasElement | null)"
          :aria-label="def.title"
          role="img"
        ></canvas>
      </div>
    </div>

    <template v-if="exportLinks">
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
    </template>

    <p v-if="partialHistory" class="text-5 mx-auto block w-4/5 md:col-span-2">
      <i class="fas fa-info-circle text-warn"></i>
      Some of this data is only captured from June 28th 2021 onwards.
    </p>
  </div>
</template>
