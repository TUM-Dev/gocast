<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";

import VideoPlayer from "@/components/player/VideoPlayer.vue";
import ReactionOverlay from "@/components/watch/ReactionOverlay.vue";
import SkipSilenceButton from "@/components/watch/SkipSilenceButton.vue";
import type { Clip } from "@/lib/player/clip";
import type { QualityPreference } from "@/lib/player/quality";
import type { SubtitleTrack } from "@/lib/player/tracks";
import { spriteSource, type WatchSilence, type WatchView } from "@/lib/watch";
import type { WatchController } from "@/lib/watch-context";
import { createProgressReporter, saveProgress } from "@/lib/watch-progress";

/**
 * One view of a stream in the video player, with what the watch page adds: the seek
 * preview sprite, subtitle tracks, the stored cut, the skip-silence button, progress
 * reporting and the highest-quality switch.
 *
 * Like VideoPlayer, everything that configures the instance is read once: the view
 * remounts this component (`:key`) when the layout or the live state changes.
 *
 * Slot `seekbar` is the strip laid over the seek bar (the bottom 30 px of the picture,
 * pointer events off for the strip itself): section dividers, bookmark markers, the
 * heatmap. It gets the `controller` and the `clip`.
 */
const props = withDefaults(
  defineProps<{
    streamId: number;
    view: WatchView;
    live: boolean;
    subtitles?: SubtitleTrack[];
    clip?: Clip;
    /** Clip seconds. */
    startTime?: number;
    rate?: number;
    playbackRates?: readonly number[];
    skipSeconds?: number;
    silences?: readonly WatchSilence[];
    /** Save the viewer's position (a signed-in viewer of a recording). */
    reportProgress?: boolean;
    /** The recording's length in seconds, which progress is a fraction of. */
    recordingDuration?: number;
  }>(),
  {
    subtitles: undefined,
    clip: undefined,
    startTime: undefined,
    rate: undefined,
    playbackRates: undefined,
    skipSeconds: 15,
    silences: () => [],
    reportProgress: false,
    recordingDuration: undefined,
  },
);

const emit = defineEmits<{
  /** The controller, once the player component exists; null when it goes away. */
  controller: [controller: WatchController | null];
}>();

const inner = ref<InstanceType<typeof VideoPlayer> | null>(null);
/** The player's root, which the seekbar overlays find the progress holder in. */
const root = ref<HTMLElement | null>(null);
const controller = ref<WatchController | null>(null);

const QUALITY_KEY = "spa.watch.highestQuality";
function readHighest(): boolean {
  try {
    return localStorage.getItem(QUALITY_KEY) === "1";
  } catch {
    return false;
  }
}
// Off by default: the best rendition is a lot of bandwidth, and adaptive streaming is
// what most viewers want.
const highest = ref(readHighest());
const quality = computed<QualityPreference>(() => (highest.value ? "highest" : "auto"));
function toggleQuality(): void {
  highest.value = !highest.value;
  try {
    localStorage.setItem(QUALITY_KEY, highest.value ? "1" : "0");
  } catch {
    // Not persisted; the choice still holds for this visit.
  }
}

const sprites = computed(() => spriteSource(props.streamId, props.view));

let stopReporting: (() => void) | null = null;
function startReporting(c: WatchController): void {
  const reporter = createProgressReporter({
    fraction() {
      const total = props.recordingDuration ?? c.getDuration() + (props.clip?.start ?? 0);
      if (!Number.isFinite(total) || total <= 0) return undefined;
      return c.toRecordingTime(c.getCurrentTime()) / total;
    },
    save: (progress) => saveProgress(props.streamId, progress),
  });
  const offs = [c.on("play", reporter.onPlay), c.on("pause", reporter.onPause), c.on("ended", reporter.onEnded)];
  stopReporting = () => {
    offs.forEach((off) => off());
    reporter.dispose();
  };
}

onMounted(() => {
  // The component instance exposes the controller API (valid before video.js loads).
  const c = inner.value as unknown as WatchController | null;
  controller.value = c;
  emit("controller", c);
  if (c && props.reportProgress && !props.live) startReporting(c);
});
onBeforeUnmount(() => {
  stopReporting?.();
  controller.value = null;
  emit("controller", null);
});
</script>

<template>
  <div ref="root" class="group relative bg-black" data-testid="watch-player">
    <VideoPlayer
      ref="inner"
      :src="view.playlistUrl"
      :live="live"
      :sprites="sprites"
      :subtitles="subtitles"
      :clip="clip"
      :start-time="startTime"
      :rate="rate"
      :playback-rates="playbackRates"
      :skip-seconds="skipSeconds"
      :quality="quality"
      persist
      hold-to-fast
      hotkeys="page"
      airplay
      stats-panel
    />
    <ReactionOverlay v-if="live" :stream-id="streamId" />
    <SkipSilenceButton v-if="silences.length > 0 && !live" :controller="controller" :silences="silences" :clip="clip" />
    <div
      class="pointer-events-none absolute right-0 bottom-0 left-0 z-10 h-[30px]"
      data-testid="region-seekbar-overlay"
    >
      <slot name="seekbar" :controller="controller" :clip="clip" :container="root" />
    </div>
    <button
      type="button"
      class="absolute top-2 left-2 z-10 rounded bg-black/60 px-2 py-1 text-xs text-white opacity-0 group-focus-within:opacity-100 group-hover:opacity-100 focus-visible:opacity-100"
      :aria-pressed="highest"
      title="Always play the highest quality available"
      data-testid="highest-quality"
      @click="toggleQuality"
    >
      Highest quality: {{ highest ? "on" : "off" }}
    </button>
  </div>
</template>
