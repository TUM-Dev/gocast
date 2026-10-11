<script setup lang="ts">
import { computed, toRef } from "vue";

import { heatmapPath, seekbarFraction, type SeekbarSection } from "@/lib/watch-actions";
import { useProgressHolder } from "./useProgressHolder";

/**
 * Section separators, bookmark markers and the seek heatmap, drawn on the video.js
 * seekbar. It renders nothing where it stands: it teleports into the player's progress
 * holder, found inside `container` (the root element of VideoPlayer.vue, i.e.
 * `playerRef.$el`).
 *
 * Positions are seconds into the recording. For a clipped player pass the clip's
 * `offset` (its start) and `duration` (the clip's length, the player's own), and items
 * outside the clip are left out. The overlay ignores pointer events so scrubbing,
 * hover previews and the player's own tooltips keep working; markers are decorative
 * and described with `title`s for those who hover the bar.
 */
const props = withDefaults(
  defineProps<{
    container: Element | null | undefined;
    /** Length of the playable range in seconds (clip length for a clipped player). */
    duration: number;
    offset?: number;
    sections?: readonly SeekbarSection[];
    bookmarks?: readonly { hours: number; minutes: number; seconds: number; description: string }[];
    /** Seek hits per bucket, from fetchSeekHeatmap. */
    heatmap?: readonly number[];
  }>(),
  { offset: 0, sections: () => [], bookmarks: () => [], heatmap: () => [] },
);

const holder = useProgressHolder(toRef(props, "container"));

// A separator sits where the next section begins; the first section has no separator
// before it, and neither does the position 0.
const separators = computed(() =>
  props.sections
    .slice(1)
    .map((s) => ({ title: s.title, at: seekbarFraction(s.start, props.duration, props.offset) }))
    .filter((s): s is { title: string; at: number } => s.at !== null && s.at > 0 && s.at < 1),
);

const markers = computed(() =>
  props.bookmarks
    .map((b) => ({
      title: b.description,
      at: seekbarFraction((b.hours * 60 + b.minutes) * 60 + b.seconds, props.duration, props.offset),
    }))
    .filter((m): m is { title: string; at: number } => m.at !== null),
);

const path = computed(() => (props.heatmap.length > 0 && props.offset === 0 ? heatmapPath(props.heatmap) : ""));
</script>

<template>
  <Teleport v-if="holder" :to="holder">
    <div class="watch-seekbar-overlay" data-testid="seekbar-overlay" aria-hidden="true">
      <svg
        v-if="path"
        class="watch-seekbar-heatmap"
        viewBox="0 0 1000 100"
        preserveAspectRatio="none"
        data-testid="seekbar-heatmap"
      >
        <path :d="path" />
      </svg>
      <span
        v-for="(s, i) in separators"
        :key="`s${i}`"
        class="watch-seekbar-separator"
        data-testid="seekbar-separator"
        :style="{ left: `${s.at * 100}%` }"
        :title="s.title"
      ></span>
      <span
        v-for="(m, i) in markers"
        :key="`b${i}`"
        class="watch-seekbar-bookmark"
        data-testid="seekbar-bookmark"
        :style="{ left: `${m.at * 100}%` }"
        :title="m.title"
      ></span>
    </div>
  </Teleport>
</template>

<style>
/* Not scoped: the nodes live inside video.js' DOM, outside this component's root. */
.watch-seekbar-overlay {
  position: absolute;
  inset: 0;
  pointer-events: none;
}
.watch-seekbar-heatmap {
  position: absolute;
  left: 0;
  bottom: 100%;
  width: 100%;
  height: 1.5rem;
  fill: rgba(255, 255, 255, 0.35);
}
.watch-seekbar-separator {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  margin-left: -1px;
  background: rgba(0, 0, 0, 0.8);
}
.watch-seekbar-bookmark {
  position: absolute;
  top: -3px;
  width: 8px;
  height: 8px;
  margin-left: -4px;
  border-radius: 9999px;
  background: #f59e0b;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.6);
}
</style>
