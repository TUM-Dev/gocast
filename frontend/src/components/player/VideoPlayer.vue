<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";

import { installClip, type Clip, type ClipHandle } from "@/lib/player/clip";
import { centrePlayToggle, labelControlsWithHotkeys, type ComponentLike } from "@/lib/player/controls";
import { createController } from "@/lib/player/controller";
import { attachHotkeys, type Hotkey } from "@/lib/player/hotkeys";
import { attachHoldSpeed } from "@/lib/player/hold-speed";
import { resetRateAtLiveEdge, type LiveTrackerLike } from "@/lib/player/live";
import type { QualityLevelListLike, QualityPreference } from "@/lib/player/quality";
import { menuRates } from "@/lib/player/speeds";
import { isMobileViewport, markTimeTooltip, spriteOptions, type SpriteSource } from "@/lib/player/sprites";
import { applyStartTime } from "@/lib/player/start-time";
import { applyStoredSettings, persistSettings, resolveRate } from "@/lib/player/storage";
import { setSubtitleTracks, type SubtitleTrack } from "@/lib/player/tracks";
import type { PlayerLike } from "@/lib/player/types";

/**
 * A video.js player for HLS recordings and live streams, shared by every SPA page that
 * plays one. The web/ frontend uses video.js too (same version), so a recording behaves
 * the same on both sides of the migration. video.js 8's bundled VHS plays HLS
 * everywhere, including Safari, where it defers to the native engine.
 *
 * video.js and its stylesheet are imported on demand: they weigh far more than the
 * rest of the app, and most pages never show a player.
 *
 * Every feature beyond a plain player is opt-in through a prop, and the defaults are the
 * plain player, so a page that passes only `src` behaves as it always has. The logic of
 * each feature lives in lib/player/*; this component only builds the video.js instance
 * and composes them. Props that configure the instance (everything except `src`, `clip`,
 * `subtitles` and `quality`) are read once, when the player is created -- remount with
 * a `:key` to change them.
 *
 * `currentTime` and `duration` are reactive and exposed, but nothing here reads them
 * in the template, so a page that only samples them on a click is not re-rendered by
 * `timeupdate` several times a second.
 *
 * A parent that drives the player (play, pause, seek, rate, events) uses the methods this
 * component exposes; they are a {@link PlayerController}, valid from before the player
 * exists. With a `clip`, every time in and out is on the clip's clock.
 */
const props = withDefaults(
  defineProps<{
    src: string;
    poster?: string;
    autoplay?: boolean;
    /** A live stream: shows the live UI and returns to 1x at the live edge. */
    live?: boolean;
    /** The speeds menu. Without it there is no menu. */
    playbackRates?: readonly number[];
    /** Seconds the skip back/forward buttons and keys jump; 0 shows no buttons. */
    skipSeconds?: number;
    /** Where to start, in seconds (the page's `?t=`). */
    startTime?: number;
    /** The speed to start at (the page's `?rate=`), over the viewer's stored one. */
    rate?: number;
    /** Keep volume, mute and speed across visits, in the old player's localStorage keys. */
    persist?: boolean;
    /** Hold the mouse on the picture to play at 2x. */
    holdToFast?: boolean;
    /** Keyboard shortcuts: while focus is in the player, or from anywhere on the page. */
    hotkeys?: false | "player" | "page";
    /** Added to (or replacing, by name) the default shortcuts. */
    extraHotkeys?: Record<string, Hotkey>;
    airplay?: boolean;
    /** "highest" pins the best rendition instead of letting the player adapt. */
    quality?: QualityPreference;
    /** Seekbar preview frames. */
    sprites?: SpriteSource;
    subtitles?: SubtitleTrack[];
    /** Plays only this part of the recording; see lib/player/clip.ts. */
    clip?: Clip;
  }>(),
  {
    poster: "",
    autoplay: false,
    live: false,
    playbackRates: undefined,
    skipSeconds: 0,
    startTime: undefined,
    rate: undefined,
    persist: false,
    holdToFast: false,
    hotkeys: false,
    extraHotkeys: undefined,
    airplay: false,
    quality: "auto",
    sprites: undefined,
    subtitles: undefined,
    clip: undefined,
  },
);

const emit = defineEmits<{
  /** Seconds, as the position moves. */
  timeupdate: [seconds: number];
  /** Seconds, once the player knows the recording's length. */
  loadedmetadata: [seconds: number];
}>();

// video.js takes over the element it is given and removes it on dispose(), which
// would pull a Vue-owned node out from under Vue. So the <video> is created by hand
// inside a host element that Vue keeps.
const host = ref<HTMLElement | null>(null);
const currentTime = ref(0);
const duration = ref(0);
const failed = ref("");

type Player = ReturnType<typeof import("video.js").default>;
let player: Player | null = null;
let mounted = false;
// Undoes whatever create() set up, in reverse order.
let cleanups: (() => void)[] = [];
let clipHandle: ClipHandle | null = null;
let subtitleCleanup: (() => void) | null = null;

const controller = createController();

const HLS = "application/x-mpegURL";

// What video.js exposes beyond its typings, as used by the composition below.
interface PlayerExtras {
  controlBar: ComponentLike;
  liveTracker?: LiveTrackerLike;
  qualityLevels?: () => QualityLevelListLike;
  spriteThumbnails?: (options: object) => void;
  getChild(name: string): ComponentLike | undefined;
}

async function create(): Promise<void> {
  const el = host.value;
  if (!el || player) return;
  const [{ default: videojs }] = await Promise.all([
    import("video.js"),
    import("video.js/dist/video-js.css"),
    import("./player.css"),
  ]);
  // Unmounted while the chunk was loading.
  if (!mounted || !host.value) return;

  const video = document.createElement("video");
  video.className = "video-js vjs-big-play-centered";
  video.setAttribute("aria-label", "Video player");
  el.appendChild(video);

  const skip = props.skipSeconds > 0 ? props.skipSeconds : 0;
  const options: Record<string, unknown> = {
    controls: true,
    fluid: true,
    preload: "metadata",
    autoplay: props.autoplay,
    poster: props.poster || undefined,
  };
  if (props.live) options.liveui = true;
  if (props.playbackRates) options.playbackRates = menuRates(props.playbackRates, videojs.browser.IS_SAFARI);
  if (skip) options.controlBar = { skipButtons: { forward: skip, backward: skip } };
  // iOS plays text in its own renderer; the tracks added below feed that one.
  if (props.subtitles) options.html5 = { nativeTextTracks: videojs.browser.IS_IOS };

  const created = videojs(video, options);
  player = created;
  const p = created as unknown as PlayerLike;
  const extras = created as unknown as PlayerExtras;

  created.on("timeupdate", () => {
    currentTime.value = created.currentTime() ?? 0;
    emit("timeupdate", currentTime.value);
  });
  created.on("loadedmetadata", () => {
    const seconds = created.duration() ?? 0;
    if (!Number.isFinite(seconds) || seconds <= 0) return;
    duration.value = seconds;
    emit("loadedmetadata", seconds);
  });
  created.on("error", () => {
    failed.value = "The recording could not be loaded.";
  });

  controller.attach(p, {
    qualityLevels: typeof extras.qualityLevels === "function" ? extras.qualityLevels() : undefined,
    liveTracker: extras.liveTracker,
  });
  controller.api.setQualityPreference(props.quality);

  // The clip goes in before anything that reads times, so they all see its clock.
  setClip(props.clip);

  if (props.persist) {
    applyStoredSettings(p, { live: props.live, rate: props.rate });
  } else if (props.rate !== undefined) {
    p.playbackRate(resolveRate(props.rate, props.live));
  }
  const hold = props.holdToFast ? attachHoldSpeed(p) : null;
  if (hold) cleanups.push(hold.dispose);
  if (props.persist) cleanups.push(persistSettings(p, { live: props.live, ignoreRate: () => !!hold?.active() }));

  if (props.live && extras.liveTracker) cleanups.push(resetRateAtLiveEdge(p, extras.liveTracker));
  if (props.startTime !== undefined && props.startTime > 0) {
    cleanups.push(applyStartTime(p, props.startTime, videojs.browser.IS_IOS));
  }
  if (skip) centrePlayToggle(extras.controlBar);
  if (props.hotkeys) {
    cleanups.push(attachHotkeys(p, { scope: props.hotkeys, seekSeconds: skip || 10, extra: props.extraHotkeys }));
    cleanups.push(labelControlsWithHotkeys(p, extras.controlBar, skip));
  }
  setSubtitles(props.subtitles);

  // Plugins that weigh something are fetched only when asked for.
  const sprite = spriteOptions(props.sprites, { mobile: isMobileViewport(), clip: props.clip });
  const pending: Promise<unknown>[] = [];
  if (sprite) {
    pending.push(
      import("videojs-sprite-thumbnails").then(() => {
        if (player !== created) return;
        extras.spriteThumbnails?.(sprite);
        markTimeTooltip(extras as never);
      }),
    );
  }
  if (props.airplay) {
    pending.push(
      Promise.all([
        import("@silvermine/videojs-airplay"),
        import("@silvermine/videojs-airplay/dist/silvermine-videojs-airplay.css"),
        import("@/lib/player/airplay"),
      ]).then(([{ default: plugin }, , { registerAirPlay, enableAirPlay }]) => {
        if (player !== created) return;
        registerAirPlay(videojs, plugin);
        enableAirPlay(created as never);
      }),
    );
  }
  if (pending.length) await Promise.all(pending);
  if (player !== created) return;
  load();
}

function setClip(clip: Clip | undefined): void {
  clipHandle?.dispose();
  clipHandle = null;
  if (player && clip) clipHandle = installClip(player as unknown as PlayerLike, clip);
}

function setSubtitles(tracks: SubtitleTrack[] | undefined): void {
  subtitleCleanup?.();
  subtitleCleanup = null;
  if (player && tracks?.length) {
    subtitleCleanup = setSubtitleTracks(player as unknown as Parameters<typeof setSubtitleTracks>[0], tracks);
  }
}

function load(): void {
  failed.value = "";
  currentTime.value = 0;
  duration.value = 0;
  if (player && props.src) player.src({ src: props.src, type: HLS });
}

function seek(seconds: number): void {
  player?.currentTime(seconds);
  currentTime.value = seconds;
}

/** The recording's own time for a time on the clip's clock (the same without a clip). */
function toRecordingTime(clipTime: number): number {
  return clipHandle ? clipHandle.toRecordingTime(clipTime) : clipTime;
}

defineExpose({ ...controller.api, currentTime, duration, seek, toRecordingTime });

onMounted(() => {
  mounted = true;
  void create();
});
watch(() => props.src, load);
watch(
  () => props.clip,
  (clip) => {
    setClip(clip);
    // Back to the new clip's start, rather than wherever the old one left the playhead.
    if (player) player.currentTime(props.startTime ?? 0);
  },
  { deep: true },
);
watch(() => props.subtitles, setSubtitles, { deep: true });
watch(() => props.quality, (quality) => controller.api.setQualityPreference(quality));
onBeforeUnmount(() => {
  mounted = false;
  subtitleCleanup?.();
  clipHandle?.dispose();
  for (const undo of cleanups.reverse()) undo();
  cleanups = [];
  controller.detach();
  player?.dispose();
  player = null;
});
</script>

<template>
  <div>
    <div ref="host" class="w-full overflow-hidden rounded bg-black" data-testid="video-player"></div>
    <p v-if="failed" class="text-5 mt-1 text-sm" role="status">{{ failed }}</p>
  </div>
</template>
