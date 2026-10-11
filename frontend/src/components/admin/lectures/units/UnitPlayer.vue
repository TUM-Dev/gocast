<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";

/**
 * The reference recording a unit's range is picked against. A browser plays HLS
 * natively only on Apple platforms, so elsewhere hls.js feeds the element; it is
 * imported on demand, as the one page that needs it should not weigh down the rest.
 *
 * The parent reads the position through `position()` rather than a reactive value:
 * it only needs it when the lecturer asks for "now", and `timeupdate` would
 * re-render the page several times a second for nothing.
 */
const props = defineProps<{ src: string }>();

const emit = defineEmits<{
  /** Seconds, once the player knows the recording's length. */
  duration: [seconds: number];
}>();

const video = ref<HTMLVideoElement | null>(null);
const failed = ref("");
let hls: { destroy(): void } | null = null;

async function attach(): Promise<void> {
  const el = video.value;
  hls?.destroy();
  hls = null;
  failed.value = "";
  if (!el || !props.src) return;

  if (el.canPlayType("application/vnd.apple.mpegurl")) {
    el.src = props.src;
    return;
  }
  const { default: Hls } = await import("hls.js");
  if (!Hls.isSupported()) {
    failed.value = "This browser cannot play the recording. Enter the times by hand.";
    return;
  }
  const player = new Hls();
  player.on(Hls.Events.ERROR, (_, data) => {
    if (data.fatal) failed.value = "The recording could not be loaded. Enter the times by hand.";
  });
  player.loadSource(props.src);
  player.attachMedia(el);
  hls = player;
}

function onMetadata(): void {
  const seconds = video.value?.duration ?? 0;
  if (Number.isFinite(seconds) && seconds > 0) emit("duration", Math.floor(seconds));
}

/** The playback position in milliseconds. */
function position(): number {
  return Math.floor((video.value?.currentTime ?? 0) * 1000);
}

function seek(ms: number): void {
  if (video.value) video.value.currentTime = ms / 1000;
}

defineExpose({ position, seek });

onMounted(attach);
watch(() => props.src, attach);
onBeforeUnmount(() => hls?.destroy());
</script>

<template>
  <div>
    <video
      ref="video"
      class="max-h-[60vh] w-full rounded bg-black"
      controls
      muted
      preload="metadata"
      aria-label="Reference recording"
      @loadedmetadata="onMetadata"
    ></video>
    <p v-if="failed" class="text-5 mt-1 text-sm" role="status">{{ failed }}</p>
  </div>
</template>
