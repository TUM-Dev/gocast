<script setup lang="ts">
import { computed } from "vue";

import { formatDuration, thumbnailUrl, type Stream, type StreamProgress } from "@/lib/streams";

/**
 * A lecture's thumbnail, with the watched bar across the bottom and the running time
 * in the corner. Ported from the repeated thumbnail markup in home.gohtml.
 *
 * The old page set this as a CSS background with a fallback image behind it, so a
 * lecture with no thumbnail yet showed the placeholder. Kept as a background for the
 * same reason: an `<img>` that 404s leaves a broken-image icon instead.
 */
const props = defineProps<{
  stream: Stream;
  slug: string;
  progress?: StreamProgress;
  /** Hides the duration and progress, for the compact rows of the list view. */
  bare?: boolean;
}>();

const FALLBACK = "/thumb-fallback.png";

const background = computed(
  () =>
    `background: url('${thumbnailUrl(props.slug, props.stream.id)}'), url('${FALLBACK}'); background-size: cover;`,
);

const percentage = computed(() => Math.min(100, Math.max(0, (props.progress?.progress ?? 0) * 100)));
</script>

<template>
  <div class="tum-live-thumbnail aspect-video" :style="background">
    <template v-if="!bare">
      <div class="tum-live-thumbnail-progress">
        <div>
          <span
            v-if="progress"
            :style="`width: ${percentage}%`"
            :class="{ 'rounded-br-lg': percentage >= 100 }"
          ></span>
        </div>
      </div>

      <div class="absolute bottom-4 right-2 z-40 flex space-x-1 text-xs text-white">
        <div v-if="progress?.watched" class="rounded-full bg-black/[.8] px-2 py-1 text-white">
          <i class="fa-solid fa-eye mr-0"></i>
        </div>
        <div v-if="stream.duration > 0" class="tum-live-badge bg-black/[.8]">
          <span>{{ formatDuration(stream.duration) }}</span>
        </div>
      </div>
    </template>
  </div>
</template>
