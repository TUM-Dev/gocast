<script setup lang="ts">
import { formatDuration } from "@/lib/streams";

/**
 * A saved lecture's picture on the offline page, looking like StreamThumbnail on the
 * course pages, with a play button that says the tile is what starts the lecture. The
 * button is always there, not only on hover: a phone has no hover. The parent puts the
 * tile in a `group`, the link it belongs to, which drives the hover and focus states.
 *
 * The image is the copy saved with the lecture (media.ts), as an object URL; a lecture
 * saved without one gets a plain tile.
 */
defineProps<{
  src?: string;
  /** The playable length, for the corner badge. */
  seconds?: number;
  /** The licence ran out: a lock instead of the play button. */
  expired?: boolean;
  /** The side list's small tiles: a smaller button, no badge. */
  compact?: boolean;
}>();
</script>

<template>
  <div class="tum-live-thumbnail mb-0 aspect-video overflow-hidden group-hover:shadow-xl group-focus-visible:shadow-xl">
    <img
      v-if="src"
      :src="src"
      alt=""
      class="absolute inset-0 h-full w-full object-cover transition-transform duration-500 group-hover:scale-105"
      :class="{ grayscale: expired }"
    />
    <div
      v-else
      class="absolute inset-0 flex items-center justify-center bg-gradient-to-br from-gray-200 to-gray-300 text-gray-400 dark:from-gray-700 dark:to-gray-800 dark:text-gray-500"
      aria-hidden="true"
    >
      <i class="fa-solid fa-graduation-cap" :class="compact ? 'text-xl' : 'text-4xl'"></i>
    </div>

    <!-- Darkens on hover so the button stands out against any picture. -->
    <div
      class="absolute inset-0 bg-black/0 transition-colors duration-300 group-hover:bg-black/30 group-focus-visible:bg-black/30"
    ></div>

    <div class="absolute inset-0 flex items-center justify-center" aria-hidden="true">
      <span
        v-if="expired"
        class="flex items-center justify-center rounded-full bg-black/70 text-white"
        :class="compact ? 'h-8 w-8 text-xs' : 'h-12 w-12'"
      >
        <i class="fa-solid fa-lock"></i>
      </span>
      <span
        v-else
        class="flex items-center justify-center rounded-full bg-black/60 text-white opacity-90 shadow-lg transition-all duration-300 group-hover:scale-110 group-hover:bg-black/80 group-hover:opacity-100 group-focus-visible:scale-110 group-focus-visible:opacity-100"
        :class="compact ? 'h-8 w-8 text-xs' : 'h-14 w-14 text-xl'"
      >
        <i class="fa-solid fa-play translate-x-px"></i>
      </span>
    </div>

    <div v-if="!compact && seconds" class="absolute right-2 bottom-2 text-xs text-white">
      <span class="tum-live-badge bg-black/[.8]">{{ formatDuration(seconds) }}</span>
    </div>
  </div>
</template>
