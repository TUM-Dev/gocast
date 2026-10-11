<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";

import VideoPlayer from "@/components/player/VideoPlayer.vue";
import { WatchViewKind, type WatchView } from "@/gen/server/apiv2_pb";
import type { Clip } from "@/lib/player/clip";
import type { PlayerController } from "@/lib/player/controller";
import type { Hotkey } from "@/lib/player/hotkeys";
import type { QualityPreference } from "@/lib/player/quality";
import type { SpriteSource } from "@/lib/player/sprites";
import type { SubtitleTrack } from "@/lib/player/tracks";
import { attachProgressReporting } from "@/lib/watch-progress";
import { deferredController } from "./controller";
import { RATIO_PRESETS, clampRatio, loadRatio, presetFor, saveRatio } from "./ratio";
import { syncPlayers } from "./sync";

/**
 * Presentation and camera side by side, in two players kept in step. The presentation
 * is the primary: it has the controls, and what the viewer does there -- play, pause,
 * seek, speed -- the camera follows. The camera has no controls and no sound.
 *
 * A parent uses this exactly as it uses a VideoPlayer: the methods exposed here are the
 * primary's {@link PlayerController}, and `currentTime`, `duration`, `seek` and
 * `toRecordingTime` are its too. The settings props are the player's, applied to the
 * primary; the camera takes only what it must share (live, clip, start, quality).
 *
 * Wide screens: a resizable pair. The divider drags, takes the arrow keys, and the
 * button offers the old page's presets (presentation only ... camera only). The ratio
 * is remembered. A side at 0% is hidden, not unmounted, so the two stay in step.
 * Phones: stacked, presentation above camera; the ratio does not apply.
 */
const props = withDefaults(
  defineProps<{
    /** StreamWatch.views; the presentation and the camera are picked from it. */
    views: readonly WatchView[];
    live?: boolean;
    clip?: Clip;
    startTime?: number;
    rate?: number;
    persist?: boolean;
    holdToFast?: boolean;
    hotkeys?: false | "player" | "page";
    extraHotkeys?: Record<string, Hotkey>;
    airplay?: boolean;
    playbackRates?: readonly number[];
    skipSeconds?: number;
    quality?: QualityPreference;
    autoplay?: boolean;
    subtitles?: SubtitleTrack[];
    /** Seekbar previews, from the presentation's sprite. */
    sprites?: SpriteSource;
    /**
     * Save the viewer's position (a signed-in viewer of a recording), from the
     * presentation, like the single player does. Needs `streamId`.
     */
    reportProgress?: boolean;
    streamId?: number;
    /** The recording's length in seconds, which progress is a fraction of. */
    recordingDuration?: number;
  }>(),
  {
    live: false,
    clip: undefined,
    startTime: undefined,
    rate: undefined,
    persist: false,
    holdToFast: false,
    hotkeys: false,
    extraHotkeys: undefined,
    airplay: false,
    playbackRates: undefined,
    skipSeconds: 0,
    quality: "auto",
    autoplay: false,
    subtitles: undefined,
    sprites: undefined,
    reportProgress: false,
    streamId: undefined,
    recordingDuration: undefined,
  },
);

const emit = defineEmits<{
  timeupdate: [seconds: number];
  loadedmetadata: [seconds: number];
  /** The presentation reached the end of the recording (or of its clip). */
  ended: [];
}>();

const presentation = computed(() => props.views.find((v) => v.kind === WatchViewKind.PRESENTATION));
const camera = computed(() => props.views.find((v) => v.kind === WatchViewKind.CAMERA));

const cam = ref(loadRatio());
const preset = computed(() => presetFor(cam.value));
const menuOpen = ref(false);

function setRatio(value: number): void {
  cam.value = clampRatio(value);
  saveRatio(cam.value);
}

const root = ref<HTMLElement | null>(null);
const row = ref<HTMLElement | null>(null);

// --- dragging the divider -------------------------------------------------------------
let dragging = false;
function onPointerDown(e: PointerEvent): void {
  dragging = true;
  (e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId);
  e.preventDefault();
}
function onPointerMove(e: PointerEvent): void {
  if (!dragging || !row.value) return;
  const box = row.value.getBoundingClientRect();
  if (box.width <= 0) return;
  cam.value = clampRatio(100 - ((e.clientX - box.left) / box.width) * 100);
}
function onPointerUp(): void {
  if (!dragging) return;
  dragging = false;
  saveRatio(cam.value);
}
function onGutterKey(e: KeyboardEvent): void {
  // The presentation is on the left, so the arrow towards it grows the camera.
  const step = e.shiftKey ? 25 : 5;
  if (e.key === "ArrowLeft") setRatio(cam.value + step);
  else if (e.key === "ArrowRight") setRatio(cam.value - step);
  else if (e.key === "Home") setRatio(0);
  else if (e.key === "End") setRatio(100);
  else return;
  e.preventDefault();
}

// --- fullscreen: the pair, not one picture ---------------------------------------------
const fullscreen = ref(false);
function onFullscreenChange(): void {
  fullscreen.value = document.fullscreenElement === root.value;
}
async function toggleFullscreen(): Promise<void> {
  if (document.fullscreenElement) await document.exitFullscreen();
  else await root.value?.requestFullscreen?.();
}

// --- the two players -----------------------------------------------------------------
const primary = ref<InstanceType<typeof VideoPlayer> | null>(null);
const follower = ref<InstanceType<typeof VideoPlayer> | null>(null);

const asController = (p: InstanceType<typeof VideoPlayer> | null) => p as unknown as PlayerController | null;
const deferred = deferredController(() => asController(primary.value));
let stopSync: (() => void) | null = null;
let stopReporting: (() => void) | null = null;

// The deferred controller keeps subscriptions across a primary that is replaced (a
// source that appears later), so this attaches once. The primary's own controller does
// the clip arithmetic, which the deferred one does not offer.
function attachEnded(): void {
  const toRecording = (t: number) => primary.value?.toRecordingTime(t) ?? t;
  if (props.reportProgress && !props.live && props.streamId !== undefined) {
    stopReporting = attachProgressReporting(
      { ...deferred.api, toRecordingTime: toRecording },
      {
        streamId: props.streamId,
        recordingDuration: props.recordingDuration,
        clipStart: props.clip?.start,
        onEnded: () => emit("ended"),
      },
    );
  } else {
    stopReporting = deferred.api.on("ended", () => emit("ended"));
  }
}

onMounted(() => {
  document.addEventListener("fullscreenchange", onFullscreenChange);
  attachEnded();
  deferred.retarget();
  const a = asController(primary.value);
  const b = asController(follower.value);
  if (a && b) stopSync = syncPlayers(a, b);
});
onBeforeUnmount(() => {
  document.removeEventListener("fullscreenchange", onFullscreenChange);
  stopSync?.();
  stopReporting?.();
});
// A source that appears later (the camera playlist of a stream that just went live)
// mounts a player after onMounted; sync it then.
watch([primary, follower], ([a, b]) => {
  stopSync?.();
  stopSync = null;
  if (a && b) {
    deferred.retarget();
    stopSync = syncPlayers(asController(a)!, asController(b)!);
  }
});

defineExpose({
  ...deferred.api,
  currentTime: computed(() => primary.value?.currentTime ?? 0),
  duration: computed(() => primary.value?.duration ?? 0),
  seek: (seconds: number) => primary.value?.seek(seconds),
  toRecordingTime: (clipTime: number) => primary.value?.toRecordingTime(clipTime) ?? clipTime,
  /** The camera, for a parent that wants to reach it. */
  follower,
  ratio: cam,
});

// Width of each side on a wide screen; 0 hides it. The gutter takes its own 10px.
const presShare = computed(() => (100 - cam.value) / 100);
const camShare = computed(() => cam.value / 100);

// --- the shared control bar --------------------------------------------------------
// The presentation's own control bar is the pair's: the stylesheet below widens it to
// the whole row (as the old page did), so its play button, seekbar, volume, speed and
// captions act on the primary and, through syncPlayers, on the camera. Two things
// video.js would otherwise do for the presentation alone are taken over here.
const presPane = ref<HTMLElement | null>(null);

// Fullscreen (the button, or a double click on the picture) is the pair's, not the
// presentation's: video.js would fullscreen only the player the bar belongs to.
function onFullscreenControl(e: MouseEvent): void {
  const target = e.target as HTMLElement | null;
  if (!target?.closest) return;
  const hit = e.type === "dblclick" ? target.closest(".vjs-tech, .vjs-poster, .vjs-text-track-display") : target.closest(".vjs-fullscreen-control");
  if (!hit || !root.value?.contains(hit)) return;
  e.stopPropagation();
  e.preventDefault();
  void toggleFullscreen();
}

// The camera cannot take the pointer (see below), so movement over it never reaches the
// presentation's player, which would hide the control bar after a moment while the
// viewer is still reaching for it. Tell the player about it.
function forwardActivity(e: MouseEvent): void {
  if (presPane.value?.contains(e.target as Node)) return;
  presPane.value?.querySelector(".video-js")?.dispatchEvent(new MouseEvent("mousemove", { clientX: e.clientX, clientY: e.clientY }));
}
</script>

<template>
  <div
    ref="root"
    class="split-root group relative flex flex-col gap-2 bg-black md:gap-0"
    :class="fullscreen ? 'split-fullscreen h-screen w-screen justify-center' : ''"
    data-testid="split-view"
    @click.capture="onFullscreenControl"
    @dblclick.capture="onFullscreenControl"
  >
    <div class="absolute right-2 top-2 z-20 flex gap-1 opacity-0 transition-opacity focus-within:opacity-100 group-hover:opacity-100">
      <div class="relative hidden md:block">
        <button
          type="button"
          class="rounded bg-black/60 px-2 py-1 text-sm text-white hover:bg-black/80"
          aria-haspopup="menu"
          :aria-expanded="menuOpen"
          title="Adjust split ratio"
          @click="menuOpen = !menuOpen"
          @keydown.esc="menuOpen = false"
        >
          <i class="fas fa-columns" aria-hidden="true"></i>
          <span class="sr-only">Adjust split ratio</span>
        </button>
        <ul
          v-if="menuOpen"
          role="menu"
          class="absolute right-0 mt-1 w-48 rounded bg-gray-900 py-1 text-sm text-white shadow-lg"
          data-testid="split-menu"
        >
          <li v-for="p in RATIO_PRESETS" :key="p.cam" role="none">
            <button
              type="button"
              role="menuitemradio"
              :aria-checked="preset?.cam === p.cam"
              class="block w-full px-3 py-1 text-left hover:bg-gray-700"
              :class="preset?.cam === p.cam ? 'font-semibold' : ''"
              @click="
                setRatio(p.cam);
                menuOpen = false;
              "
            >
              {{ p.label }}
            </button>
          </li>
        </ul>
      </div>
    </div>

    <div
      ref="row"
      class="split-row flex flex-col gap-2 md:flex-row md:gap-0"
      :style="{ '--g': presentation && camera ? '0.625rem' : '0px' }"
      @mousemove="forwardActivity"
    >
      <div
        v-if="presentation"
        ref="presPane"
        class="split-primary min-w-0"
        :class="cam === 100 ? 'split-primary-hidden' : ''"
        :style="{ '--r': presShare }"
        :aria-hidden="cam === 100 ? 'true' : undefined"
        data-testid="split-presentation"
      >
        <VideoPlayer
          ref="primary"
          :src="presentation.playlistUrl"
          :live="live"
          :clip="clip"
          :start-time="startTime"
          :rate="rate"
          :persist="persist"
          :hold-to-fast="holdToFast"
          :hotkeys="hotkeys"
          :extra-hotkeys="extraHotkeys"
          :airplay="airplay"
          :playback-rates="playbackRates"
          :skip-seconds="skipSeconds"
          :quality="quality"
          :autoplay="autoplay"
          :subtitles="subtitles"
          :sprites="sprites"
          @timeupdate="emit('timeupdate', $event)"
          @loadedmetadata="emit('loadedmetadata', $event)"
        />
      </div>

      <!-- Only where the two sit side by side; stacked, there is nothing to resize. -->
      <div
        v-if="presentation && camera"
        class="hidden w-2.5 shrink-0 cursor-col-resize touch-none bg-gray-700 hover:bg-gray-500 focus-visible:bg-blue-500 md:block"
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize presentation and camera"
        :aria-valuenow="100 - cam"
        aria-valuemin="0"
        aria-valuemax="100"
        tabindex="0"
        data-testid="split-gutter"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerUp"
        @keydown="onGutterKey"
      ></div>

      <!--
        No controls, and no pointer: a click that paused only this side would pull the
        two apart. Everything the viewer does, they do on the presentation.
      -->
      <div
        v-if="camera"
        class="split-follower pointer-events-none min-w-0 overflow-hidden"
        :style="{ '--r': camShare }"
        :aria-hidden="cam === 0 ? 'true' : undefined"
        data-testid="split-camera"
      >
        <VideoPlayer ref="follower" :src="camera.playlistUrl" :live="live" :clip="clip" :start-time="startTime" :quality="quality" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.split-follower :deep(.vjs-control-bar),
.split-follower :deep(.vjs-big-play-button),
.split-follower :deep(.vjs-loading-spinner) {
  display: none !important;
}

/*
 * The presentation's control bar spans the whole row, not just its own pane: 100cqw is
 * the width of .split-root, a size container. The pane and the player's host must not
 * clip it, and it must sit above the camera, which comes later in the DOM.
 */
.split-root {
  container-type: inline-size;
}
.split-primary :deep([data-testid="video-player"]) {
  overflow: visible;
}
.split-primary :deep(.vjs-control-bar) {
  width: 100cqw;
  max-width: none;
  z-index: 5;
}
/* With the presentation at 0% only its control bar is left to show. */
.split-primary-hidden :deep(.vjs-tech),
.split-primary-hidden :deep(.vjs-poster),
.split-primary-hidden :deep(.vjs-big-play-button),
.split-primary-hidden :deep(.vjs-text-track-display),
.split-primary-hidden :deep(.vjs-loading-spinner) {
  display: none;
}

@media (min-width: 768px) {
  /*
   * Side by side. The two shares split what the gutter leaves, so the row adds up to
   * 100% instead of overflowing by the gutter's width. The row has the picture's 16:9
   * shape and the players fill it (video.js' fluid mode would give each its own height
   * from its own width and leave the control bar at the bottom of the shorter one), so
   * the bar sits at the bottom of the pair and each picture is centred in its side.
   */
  .split-row {
    aspect-ratio: 16 / 9;
  }
  .split-fullscreen .split-row {
    aspect-ratio: auto;
    flex: 1 1 0;
    min-height: 0;
  }
  .split-primary,
  .split-follower {
    flex: 0 0 calc((100% - var(--g)) * var(--r));
    height: 100%;
  }
  .split-primary > :deep(div),
  .split-follower > :deep(div),
  .split-primary :deep([data-testid="video-player"]),
  .split-follower :deep([data-testid="video-player"]),
  .split-primary :deep(.video-js),
  .split-follower :deep(.video-js) {
    height: 100%;
  }
  .split-primary :deep(.video-js.vjs-fluid),
  .split-follower :deep(.video-js.vjs-fluid) {
    height: 100% !important;
    padding-top: 0 !important;
  }
}
</style>
