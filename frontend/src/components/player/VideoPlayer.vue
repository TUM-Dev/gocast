<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";

/**
 * A video.js player for HLS recordings, shared by every SPA page that plays one. The
 * web/ frontend uses video.js too (same version), so a recording behaves the same on
 * both sides of the migration. video.js 8's bundled VHS plays HLS everywhere,
 * including Safari, where it defers to the native engine.
 *
 * video.js and its stylesheet are imported on demand: they weigh far more than the
 * rest of the app, and most pages never show a player.
 *
 * `currentTime` and `duration` are reactive and exposed, but nothing here reads them
 * in the template, so a page that only samples them on a click is not re-rendered by
 * `timeupdate` several times a second.
 */
const props = withDefaults(defineProps<{ src: string; poster?: string; autoplay?: boolean }>(), {
  poster: "",
  autoplay: false,
});

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

const HLS = "application/x-mpegURL";

async function create(): Promise<void> {
  const el = host.value;
  if (!el || player) return;
  const [{ default: videojs }] = await Promise.all([import("video.js"), import("video.js/dist/video-js.css")]);
  // Unmounted while the chunk was loading.
  if (!mounted || !host.value) return;

  const video = document.createElement("video");
  video.className = "video-js vjs-big-play-centered";
  video.setAttribute("aria-label", "Video player");
  el.appendChild(video);

  player = videojs(video, {
    controls: true,
    fluid: true,
    preload: "metadata",
    autoplay: props.autoplay,
    poster: props.poster || undefined,
  });
  player.on("timeupdate", () => {
    currentTime.value = player?.currentTime() ?? 0;
    emit("timeupdate", currentTime.value);
  });
  player.on("loadedmetadata", () => {
    const seconds = player?.duration() ?? 0;
    if (!Number.isFinite(seconds) || seconds <= 0) return;
    duration.value = seconds;
    emit("loadedmetadata", seconds);
  });
  player.on("error", () => {
    failed.value = "The recording could not be loaded.";
  });
  load();
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

defineExpose({ currentTime, duration, seek });

onMounted(() => {
  mounted = true;
  void create();
});
watch(() => props.src, load);
onBeforeUnmount(() => {
  mounted = false;
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
