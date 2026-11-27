<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { RouterLink, useRoute } from "vue-router";

import { useOfflineStore } from "@/stores/offline";

/**
 * How saving a lecture offline is going, for as long as it runs and briefly after. The
 * save buttons sit in menus that close on the click (the watch page's downloads, a
 * lecture card's), so their own label is gone before anything happens. Mounted once by
 * the app shell like DownloadNotice, so it follows the viewer to other pages while the
 * download goes on. Left out on /offline, which shows the same progress itself.
 */
const offline = useOfflineStore();
const route = useRoute();

const active = computed(() => Object.keys(offline.saving).map(Number));
const single = computed(() => (active.value.length === 1 ? active.value[0] : undefined));
const fraction = computed(() => (single.value === undefined ? undefined : offline.progress[single.value]));

const quoted = (title: string | undefined) => (title ? `“${title}”` : "the lecture");

const savingText = computed(() => {
  if (single.value === undefined) return `Saving ${active.value.length} lectures offline…`;
  const title = quoted(offline.byStream(single.value)?.title);
  return fraction.value === undefined ? `Preparing ${title} for offline viewing…` : `Saving ${title} offline…`;
});

/** The ended save on show; errors stay longer, since they ask the viewer to act. */
const ended = ref<typeof offline.finished>(null);
let timer: ReturnType<typeof setTimeout> | undefined;
watch(
  () => offline.finished,
  (f) => {
    ended.value = f;
    clearTimeout(timer);
    if (f) timer = setTimeout(() => (ended.value = null), f.error ? 10_000 : 6_000);
  },
);
onBeforeUnmount(() => clearTimeout(timer));

function dismiss(): void {
  clearTimeout(timer);
  ended.value = null;
}

const mode = computed(() => {
  if (route.name === "offline") return null;
  if (active.value.length > 0) return "saving";
  if (ended.value) return ended.value.error ? "failed" : "done";
  return null;
});
</script>

<template>
  <div
    v-if="mode === 'saving'"
    role="status"
    data-testid="offline-save-notice"
    class="tum-live-notification tum-live-notification-info fixed bottom-4 left-1/2 z-50 flex w-[calc(100%-2rem)] max-w-md -translate-x-1/2 flex-col gap-1 py-2 shadow-lg"
  >
    <div class="flex items-center">
      <i class="icon fa-solid fa-cloud-arrow-down"></i>
      <span class="title min-w-0 grow truncate">{{ savingText }}</span>
      <span v-if="fraction !== undefined" class="ml-2 shrink-0 tabular-nums">{{ Math.floor(fraction * 100) }} %</span>
    </div>
    <!-- Without a fraction yet (the runner is packaging), the bar pulses instead. -->
    <div class="h-1 w-full overflow-hidden rounded-full bg-gray-500/20">
      <div
        class="tum-live-bg-primary h-full rounded-full transition-[width] duration-300"
        :class="{ 'animate-pulse': fraction === undefined }"
        :style="{ width: fraction === undefined ? '100%' : `${fraction * 100}%` }"
      ></div>
    </div>
    <span class="text-5 text-xs">Keep the app open until it's done.</span>
  </div>

  <div
    v-else-if="mode === 'failed'"
    role="alert"
    data-testid="offline-save-failed"
    class="tum-live-notification tum-live-notification-warn fixed bottom-4 left-1/2 z-50 flex w-[calc(100%-2rem)] max-w-md -translate-x-1/2 items-center shadow-lg"
  >
    <i class="icon fa-solid fa-triangle-exclamation"></i>
    <span class="title">{{ ended?.error }}</span>
    <button type="button" class="ml-auto px-2" aria-label="Dismiss" @click="dismiss">
      <i class="fa-solid fa-xmark"></i>
    </button>
  </div>

  <div
    v-else-if="mode === 'done'"
    role="status"
    data-testid="offline-save-done"
    class="tum-live-notification tum-live-notification-info fixed bottom-4 left-1/2 z-50 flex w-[calc(100%-2rem)] max-w-md -translate-x-1/2 items-center shadow-lg"
  >
    <i class="icon fa-solid fa-circle-check text-green-600 dark:text-green-400"></i>
    <span class="title min-w-0 truncate">Saved {{ quoted(ended?.title) }} offline.</span>
    <RouterLink to="/offline" class="ml-auto shrink-0 px-2 font-semibold whitespace-nowrap" @click="dismiss">Open</RouterLink>
    <button type="button" class="shrink-0 px-2" aria-label="Dismiss" @click="dismiss">
      <i class="fa-solid fa-xmark"></i>
    </button>
  </div>
</template>
