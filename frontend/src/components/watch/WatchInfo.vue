<script setup lang="ts">
import { computed } from "vue";
import { RouterLink } from "vue-router";

import { courseHref, formatStreamTime, streamStart, type StreamWatch, type WatchPhase } from "@/lib/watch";

/**
 * The lecture's title, its course, and the status line under it: a LIVE badge with the
 * viewer count while it is broadcast, the date of a recording, "waiting" before the
 * start and "ended" after.
 *
 * `title` and `viewers` are the live values (realtime events) and win over what the
 * page data said when it was loaded.
 */
const props = defineProps<{
  watch: StreamWatch;
  phase: WatchPhase;
  title?: string;
  viewers: number;
}>();

const heading = computed(() => props.title || props.watch.stream?.displayName || "");
const href = computed(() => courseHref(props.watch));
const when = computed(() => {
  const start = streamStart(props.watch);
  return start ? formatStreamTime(start) : "";
});
</script>

<template>
  <div class="flex flex-col gap-1">
    <h1 class="text-1 line-clamp-2 text-lg font-bold lg:text-2xl" data-testid="watch-title">{{ heading }}</h1>
    <p v-if="watch.course" class="text-sm lg:text-base">
      <RouterLink v-if="href" :to="href" class="text-5 hover:text-1 hover:underline" data-testid="watch-course">
        {{ watch.course.name }}
      </RouterLink>
      <span v-else class="text-5">{{ watch.course.name }}</span>
    </p>
    <p class="text-5 flex flex-wrap items-center gap-x-2 text-xs lg:text-sm" data-testid="watch-status">
      <template v-if="phase === 'live'">
        <span class="rounded bg-red-600 px-2 py-0.5 text-xs font-semibold text-white" data-testid="live-badge">LIVE</span>
        <span data-testid="viewers">{{ viewers }} watching</span>
      </template>
      <template v-else-if="phase === 'waiting'">
        <span data-testid="viewers">{{ viewers }} waiting</span>
      </template>
      <template v-else-if="phase === 'ended'">
        <span>Stream has ended</span>
        <span v-if="when" aria-hidden="true">&middot;</span>
        <span v-if="when">{{ when }}</span>
      </template>
      <template v-else>
        <span v-if="when">{{ when }}</span>
        <span v-if="watch.stream?.lectureHallName" aria-hidden="true">&middot;</span>
        <span v-if="watch.stream?.lectureHallName">{{ watch.stream.lectureHallName }}</span>
      </template>
    </p>
  </div>
</template>
