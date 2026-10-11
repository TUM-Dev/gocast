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
  },
);

const emit = defineEmits<{
  timeupdate: [seconds: number];
  loadedmetadata: [seconds: number];
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

onMounted(() => {
  document.addEventListener("fullscreenchange", onFullscreenChange);
  deferred.retarget();
  const a = asController(primary.value);
  const b = asController(follower.value);
  if (a && b) stopSync = syncPlayers(a, b);
});
onBeforeUnmount(() => {
  document.removeEventListener("fullscreenchange", onFullscreenChange);
  stopSync?.();
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
const presWidth = computed(() => `${100 - cam.value}%`);
const camWidth = computed(() => `${cam.value}%`);
</script>

<template>
  <div
    ref="root"
    class="group relative flex flex-col gap-2 bg-black md:gap-0"
    :class="fullscreen ? 'h-screen w-screen justify-center' : ''"
    data-testid="split-view"
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
      <button
        type="button"
        class="rounded bg-black/60 px-2 py-1 text-sm text-white hover:bg-black/80"
        :title="fullscreen ? 'Exit fullscreen' : 'Fullscreen'"
        @click="toggleFullscreen"
      >
        <i class="fas" :class="fullscreen ? 'fa-compress' : 'fa-expand'" aria-hidden="true"></i>
        <span class="sr-only">{{ fullscreen ? "Exit fullscreen" : "Fullscreen" }}</span>
      </button>
    </div>

    <div ref="row" class="flex flex-col gap-2 md:flex-row md:gap-0">
      <div
        v-if="presentation"
        class="min-w-0 overflow-hidden md:[flex:0_0_var(--w)]"
        :style="{ '--w': presWidth }"
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
        class="split-follower pointer-events-none min-w-0 overflow-hidden md:[flex:0_0_var(--w)]"
        :style="{ '--w': camWidth }"
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
</style>
