<script setup lang="ts">
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch as watchEffect } from "vue";
import { useRoute } from "vue-router";

import VideoPlayer from "@/components/player/VideoPlayer.vue";
import type { StreamWatch } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import type { PlayerController } from "@/lib/player/controller";
import { DEFAULT_PLAYBACK_RATES } from "@/lib/player/speeds";
import { RealtimeConnection } from "@/lib/realtime";
import { fetchStreamWatch } from "@/lib/watch";
import {
  embedClip,
  embedKind,
  embedSprite,
  minutesUntil,
  pickEmbedView,
  waitingForStart,
} from "@/lib/watch-admin-embed";

/**
 * The player on its own, for a page that frames it (Artemis does). No header, no
 * footer, no chat; a title bar that links back to the lecture, and while the lecture
 * has not begun, a notice of when it will.
 *
 * Which picture plays comes from the URL's version (`COMB`, `PRES`, `CAM`). The page's
 * data is the parent's to hand over as `watch` -- the watch page already loaded it --
 * and is fetched here, with or without a session, when it is not. Anonymous viewers of a
 * public course are served: that is what an embed in someone else's page mostly is.
 *
 * Covers the window (`fixed inset-0`) so that it is bare even where the app shell still
 * draws its header; the shell should also drop the header for `?video_only`, but a
 * missed integration then costs nothing visible.
 */
const props = defineProps<{
  /** The loaded StreamWatch, when the parent has it. */
  watch?: StreamWatch;
}>();

// Served by Go, not bundled: a constant keeps the template compiler from resolving it as an import.
const BANNER = "/public/default_banner.jpg";

const route = useRoute();
const loaded = shallowRef<StreamWatch | undefined>(undefined);
const data = computed(() => props.watch ?? loaded.value);
const error = ref("");
const now = ref(new Date());

const slug = computed(() => String(route.params.slug ?? ""));
const streamId = computed(() => Number(route.params.streamID));

async function load(): Promise<void> {
  try {
    loaded.value = await fetchStreamWatch(slug.value, streamId.value);
    error.value = "";
  } catch (err) {
    loaded.value = undefined;
    error.value =
      err instanceof ApiError && (err.status === 401 || err.status === 403 || err.status === 404)
        ? "This video is not available."
        : "The video could not be loaded.";
  }
}

const view = computed(() =>
  data.value ? pickEmbedView(data.value.views, embedKind(String(route.params.version ?? ""))) : undefined,
);
const stream = computed(() => data.value?.stream);
const course = computed(() => data.value?.course);
const clip = computed(() => (data.value ? embedClip(data.value) : undefined));
const sprites = computed(() => embedSprite(streamId.value, view.value));
const start = computed(() => (stream.value?.start ? timestampDate(stream.value.start) : null));
const waiting = computed(() => !!data.value && waitingForStart(data.value));
const minutes = computed(() => minutesUntil(start.value, now.value));
const startTime = computed(() => {
  const t = Number(Array.isArray(route.query.t) ? route.query.t[0] : route.query.t);
  return Number.isFinite(t) && t > 0 ? t : undefined;
});

const lectureUrl = computed(() => `/w/${encodeURIComponent(course.value?.slug ?? slug.value)}/${streamId.value}`);
const courseUrl = computed(() => (course.value ? `/course/${course.value.year}/${course.value.term}/${encodeURIComponent(course.value.slug)}` : "/"));

// The title bar shows once playback has begun (so it never covers the big play button),
// and then while paused or while the viewer is active.
const started = ref(false);
const paused = ref(true);
const active = ref(false);
let idle: ReturnType<typeof setTimeout> | undefined;
const ACTIVE_MS = 3000;
const showTitle = computed(() => started.value && (paused.value || active.value));

function touch(): void {
  active.value = true;
  clearTimeout(idle);
  idle = setTimeout(() => (active.value = false), ACTIVE_MS);
}

const player = ref<PlayerController | null>(null);
const unsubscribe: (() => void)[] = [];
watchEffect(player, (p) => {
  unsubscribe.splice(0).forEach((fn) => fn());
  if (!p) return;
  unsubscribe.push(
    p.on("play", () => {
      started.value = true;
      paused.value = false;
    }),
    p.on("pause", () => (paused.value = true)),
  );
});

// A lecture that goes live, or ends, is fetched again: the playlist changes with it.
let connection: RealtimeConnection | null = null;
let tick: ReturnType<typeof setInterval> | undefined;

onMounted(async () => {
  if (!props.watch) await load();
  // The notice counts in whole minutes; once every ten seconds is plenty.
  tick = setInterval(() => (now.value = new Date()), 10_000);
  if (!Number.isInteger(streamId.value) || streamId.value <= 0) return;
  connection = new RealtimeConnection({ streamId: streamId.value });
  connection.on("live", () => void load());
  connection.on("resync", () => void load());
  connection.connect();
});
onBeforeUnmount(() => {
  clearInterval(tick);
  clearTimeout(idle);
  unsubscribe.splice(0).forEach((fn) => fn());
  connection?.close();
});
</script>

<template>
  <div
    class="fixed inset-0 z-50 flex items-center justify-center overflow-hidden bg-black"
    data-testid="watch-embed"
    @mousemove="touch"
    @focusin="touch"
  >
    <p v-if="error" class="px-4 text-center text-white" role="alert">{{ error }}</p>

    <div v-else-if="data && stream" class="relative h-full w-full">
      <div class="flex h-full w-full items-center justify-center">
        <VideoPlayer
          v-if="view && !waiting"
          :key="view.playlistUrl"
          ref="player"
          class="w-full"
          :src="view.playlistUrl"
          :live="stream.liveNow"
          :clip="clip"
          :sprites="sprites"
          :start-time="startTime"
          :playback-rates="DEFAULT_PLAYBACK_RATES"
          :skip-seconds="10"
          persist
          hotkeys="player"
          poster="/public/default_banner.jpg"
        />
        <img v-else :src="BANNER" alt="" class="max-h-full max-w-full object-contain" />
      </div>

      <!-- Over the picture, not in the player, which has no title bar to offer. -->
      <div
        v-show="showTitle"
        class="pointer-events-none absolute inset-x-0 top-0 z-10 bg-gradient-to-b from-black/75 to-transparent px-2 pb-10 pt-2"
        data-testid="embed-title"
      >
        <div class="flex gap-2">
          <div class="min-w-0 grow">
            <h1 class="truncate text-2xl">
              <a :href="lectureUrl" target="_blank" rel="noopener" class="pointer-events-auto text-gray-100 hover:text-white hover:underline">{{ stream.displayName }}</a>
            </h1>
            <h2 class="truncate font-semibold">
              <a :href="courseUrl" target="_blank" rel="noopener" class="pointer-events-auto text-gray-100 hover:text-white hover:underline">{{ course?.name }}</a>
            </h2>
          </div>
          <a :href="lectureUrl" target="_blank" rel="noopener" class="pointer-events-auto shrink-0 text-gray-200 hover:text-white hover:underline">
            TUM-Live <i class="fas fa-external-link-alt" aria-hidden="true"></i>
          </a>
        </div>
      </div>

      <div
        v-if="waiting && minutes > 0"
        class="absolute bottom-4 left-3 z-10 rounded bg-gray-900/75 p-4 text-white"
        role="status"
        data-testid="embed-starts-in"
      >
        <p>
          <a :href="lectureUrl" target="_blank" rel="noopener" class="font-semibold text-gray-300 hover:text-white hover:underline">{{ stream.displayName }}</a>
        </p>
        <p>
          <a :href="courseUrl" target="_blank" rel="noopener" class="text-sm text-gray-300 hover:text-white hover:underline">{{ course?.name }}</a>
        </p>
        <p class="text-sm">Start in about <span class="font-semibold">{{ minutes }}</span> {{ minutes === 1 ? "minute" : "minutes" }}</p>
      </div>
    </div>
  </div>
</template>
