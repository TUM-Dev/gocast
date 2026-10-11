<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";

import { countdown, formatCountdown, formatStreamTime } from "@/lib/watch";

/**
 * Where a viewer waits for a lecture that has not started: the countdown, ticking each
 * second, and the scheduled time. Once the start has passed it says the stream is due;
 * the page polls the server (and the realtime live event) to swap in the player, this
 * only displays.
 */
const props = defineProps<{ start: Date; title: string }>();

const now = ref(new Date());
let timer: ReturnType<typeof setInterval> | undefined;
onMounted(() => (timer = setInterval(() => (now.value = new Date()), 1000)));
onBeforeUnmount(() => clearInterval(timer));

const remaining = computed(() => countdown(props.start, now.value));
const text = computed(() => formatCountdown(remaining.value));
</script>

<template>
  <div
    class="flex aspect-video w-full flex-col items-center justify-center gap-2 bg-gray-900 p-6 text-center text-white"
    data-testid="waiting-room"
  >
    <p class="text-sm text-gray-300">{{ title }}</p>
    <p class="text-4xl font-semibold tabular-nums lg:text-6xl" role="timer" aria-live="off" data-testid="countdown">{{ text }}</p>
    <p class="text-sm text-gray-300">Scheduled for {{ formatStreamTime(start) }}</p>
    <p class="text-xs text-gray-400">This page switches to the stream when it starts.</p>
  </div>
</template>
