<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";

import { streamFileUrl } from "@/lib/stream-files";
import { currentIndex, fetchSections, formatTime, type PanelPlayer, type WatchSection } from "@/lib/watch-panels";

/**
 * The lecture's chapters: a picture and a title each, the one playing highlighted,
 * click to jump. Follows playback by scrolling the current one into view; scrolling
 * by hand is respected for as long as "Follow playback" is off.
 *
 * Emits `sections` once loaded, for chapter marks on the seekbar.
 */
const props = defineProps<{ streamId: number; slug: string; player: PanelPlayer }>();
const emit = defineEmits<{
  sections: [sections: WatchSection[]];
}>();

const sections = ref<WatchSection[]>([]);
const current = ref(-1);
const loaded = ref(false);
const error = ref("");
const follow = ref(true);
const items = ref<HTMLElement[]>([]);

let off: (() => void) | undefined;

function sync(): void {
  current.value = currentIndex(sections.value, props.player.getCurrentTime());
}

onMounted(async () => {
  off = props.player.on("timeupdate", sync);
  try {
    sections.value = await fetchSections(props.slug, props.streamId);
    emit("sections", sections.value);
    sync();
  } catch (err) {
    error.value = err instanceof Error ? err.message : "Could not load the sections.";
  } finally {
    loaded.value = true;
  }
});
onBeforeUnmount(() => off?.());

// Only when the section changes, not on every tick: scrolling once a second would
// fight a viewer who is reading further down.
watch(current, async (i) => {
  if (!follow.value || i < 0) return;
  await nextTick();
  items.value[i]?.scrollIntoView?.({ block: "nearest", behavior: "smooth" });
});
</script>

<template>
  <section class="flex h-full min-h-0 flex-col" aria-label="Sections">
    <header class="flex items-center justify-between p-3">
      <h3 class="text-3 text-base font-bold">Sections</h3>
      <button
        type="button"
        class="text-3 rounded-full px-3 py-1 text-sm hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-blue-500 dark:hover:bg-gray-600"
        :class="{ 'bg-gray-100 font-semibold dark:bg-gray-700': follow }"
        :aria-pressed="follow"
        @click="follow = !follow"
      >
        Follow playback
      </button>
    </header>

    <p v-if="error" class="px-3 text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
    <p v-else-if="loaded && !sections.length" class="text-5 px-3 text-sm">This lecture has no sections.</p>

    <ul class="min-h-0 grow overflow-y-auto">
      <li v-for="(s, i) in sections" :key="`${s.time}-${s.fileId}`" :ref="(el) => (items[i] = el as HTMLElement)">
        <button
          type="button"
          class="flex w-full items-center gap-3 px-3 py-2 text-left hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-blue-500 dark:hover:bg-gray-600"
          :class="{ 'bg-gray-200 dark:bg-gray-700': i === current }"
          :aria-current="i === current ? 'true' : undefined"
          @click="player.seek(s.time)"
        >
          <span class="relative block aspect-video h-14 shrink-0 overflow-hidden rounded-lg bg-gray-100 dark:bg-gray-800">
            <img
              v-if="s.fileId"
              :src="streamFileUrl(streamId, s.fileId)"
              alt=""
              loading="lazy"
              class="h-full w-full object-cover"
            />
            <span
              class="absolute bottom-1 right-1 rounded bg-sky-200 px-1 text-xs text-sky-800 dark:bg-indigo-800 dark:text-indigo-200"
              >{{ formatTime(s.time) }}</span
            >
          </span>
          <span class="text-3 min-w-0 break-words text-xs font-semibold">{{ s.description }}</span>
        </button>
      </li>
    </ul>
  </section>
</template>
