<script setup lang="ts">
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from "vue";
import { useRoute } from "vue-router";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import ChatPanel from "@/components/chat/ChatPanel.vue";
import VideoPlayer from "@/components/player/VideoPlayer.vue";
import CameraPresets from "@/components/watch/admin/CameraPresets.vue";
import EndStreamDialog from "@/components/watch/admin/EndStreamDialog.vue";
import type { StreamWatch } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import {
  adminLinks,
  clock,
  reactionPercentages,
  timeLeft,
} from "@/lib/watch-admin";
import { fetchStreamWatch } from "@/lib/watch";
import { fetchAllowedReactions } from "@/lib/watch-actions";
import { useLiveEvents } from "@/lib/watch-admin-live";
import { pickEmbedView } from "@/lib/watch-admin-embed";
import { WatchViewKind } from "@/gen/server/apiv2_pb";
import { redirectToLogin, useAuthStore } from "@/stores/auth";
import { useCourseAdminStore } from "@/stores/course-admin";
import { useStreamRealtime } from "@/stores/streamRealtime";

/**
 * The lecturer's view of a running lecture: the live picture (muted, at the highest
 * quality, with a way back to the live edge), how many are watching, the time left, how
 * the audience is reacting, the camera presets, the chat, and "End stream".
 *
 * The server refuses anyone who does not administer the course before serving this page,
 * and getStreamWatch leaves out the admin controls for anyone else; a stream that is
 * not live has nothing to manage and says so.
 */
const auth = useAuthStore();
const route = useRoute();
const courseStore = useCourseAdminStore();

const courseId = computed(() => Number(route.params.courseID));
const streamId = computed(() => Number(route.params.streamID));

const data = shallowRef<StreamWatch | undefined>(undefined);
const loading = ref(true);
const error = ref("");
const allowed = ref<string[]>([]);
const ending = ref(false);
const player = ref<{ seekToLive(): void } | null>(null);
const now = ref(new Date());

// One socket per tab: the chat panel on this page listens to the same one.
const realtime = useStreamRealtime(streamId.value);
let firstSync = true;
const events = useLiveEvents(streamId.value, () => void reload(), (handler) =>
  realtime.subscribe({
    onEvent: handler,
    // The first subscribe is a resync too, but the page has just loaded.
    onResync: () => (firstSync ? (firstSync = false) : void reload()),
  }),
);

const stream = computed(() => data.value?.stream);
const admin = computed(() => data.value?.admin);
const live = computed(() => events.live.value ?? stream.value?.liveNow ?? false);
const end = computed(() => events.end.value ?? (stream.value?.end ? timestampDate(stream.value.end) : null));
const viewers = computed(() => events.viewers.value ?? data.value?.viewers ?? 0);
const view = computed(() =>
  data.value ? pickEmbedView(data.value.views, WatchViewKind.COMBINED) : undefined,
);
const links = computed(() => adminLinks(courseId.value, streamId.value, data.value?.course?.slug ?? ""));
const reactions = computed(() => reactionPercentages(events.stats.value, allowed.value.length ? allowed.value : undefined));

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403 || err.status === 404) return "This lecture does not exist or you do not administer it.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

let inFlight = false;
async function reload(): Promise<void> {
  if (inFlight) return;
  const slug = courseStore.course?.id === courseId.value ? courseStore.course.slug : "";
  if (!slug) return;
  inFlight = true;
  try {
    const loaded = await fetchStreamWatch(slug, streamId.value);
    // The stream must be this course's: the slug in the request decides, and a lecture
    // of another course is refused by the server, but do not trust that alone.
    if (loaded.course?.id !== courseId.value) throw new ApiError(404, "not this course's lecture");
    data.value = loaded;
    error.value = "";
  } catch (err) {
    data.value = undefined;
    error.value = message(err);
  } finally {
    inFlight = false;
    loading.value = false;
  }
}

watch(
  courseId,
  async (id) => {
    const user = await auth.load().catch(() => null);
    if (!user) {
      redirectToLogin();
      return;
    }
    loading.value = true;
    await courseStore.load(id);
    if (!courseStore.course) {
      error.value = courseStore.error || "This lecture does not exist or you do not administer it.";
      loading.value = false;
      return;
    }
    await reload();
    fetchAllowedReactions(streamId.value)
      .then((r) => (allowed.value = r))
      .catch(() => {});
  },
  { immediate: true },
);

// The recording appears (or the stream starts) when the live flag flips.
// Watching the event's value, not the derived `live`: that one also moves when the
// first load lands, which is no news.
watch(events.live, (flag) => {
  if (flag !== undefined) void reload();
});

let tick: ReturnType<typeof setInterval> | undefined;
onMounted(() => (tick = setInterval(() => (now.value = new Date()), 1000)));
onBeforeUnmount(() => clearInterval(tick));
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex w-full max-w-7xl flex-col gap-4">
      <header>
        <h1 class="text-1 text-2xl font-bold">
          <RouterLink v-if="data" :to="links.watch" class="hover:underline">{{ stream?.displayName }}</RouterLink>
          <template v-else>Live management</template>
        </h1>
        <p v-if="data" class="text-3">
          <RouterLink :to="`/admin/courses/${courseId}/lectures`" class="hover:underline">{{ data.course?.name }}</RouterLink>
          ·
          <RouterLink :to="links.stats" class="hover:underline">Statistics</RouterLink>
        </p>
      </header>

      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>
      <p v-else-if="loading" class="text-5 text-sm">Loading…</p>

      <p v-else-if="data && !live" class="text-3 rounded-lg bg-gray-100 px-4 py-6 dark:bg-gray-800" role="status" data-testid="not-live">
        This lecture is not live right now, so there is nothing to manage.
        <RouterLink :to="links.watch" class="underline">Go to the lecture</RouterLink>
      </p>

      <div v-else-if="data && view" class="grid gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <div class="flex min-w-0 flex-col gap-3">
          <div class="text-3 flex flex-wrap items-center justify-between gap-2 text-sm">
            <span class="flex items-center gap-2">
              <span class="inline-block h-2.5 w-2.5 animate-pulse rounded-full bg-red-500" aria-hidden="true"></span>
              <span data-testid="time-left">Time left: {{ timeLeft(end, now) }}</span>
            </span>
            <span data-testid="viewers"><i class="fas fa-eye" aria-hidden="true"></i> {{ viewers }} watching</span>
            <span data-testid="clock">{{ clock(now) }}</span>
          </div>

          <VideoPlayer ref="player" :src="view.playlistUrl" live autoplay quality="highest" airplay stats-panel hotkeys="player" poster="/public/no_active_stream.jpg" />

          <button
            type="button"
            class="rounded-2xl bg-indigo-600 px-3 py-1 font-medium text-white hover:bg-indigo-500"
            @click="player?.seekToLive()"
          >
            Seek to Live
          </button>

          <div v-if="stream?.descriptionHtml" class="text-3 rounded-lg bg-gray-50 p-2 text-sm dark:bg-secondary-light" v-html="stream.descriptionHtml"></div>

          <CameraPresets v-if="admin" :course-id="courseId" :stream-id="streamId" :presets="admin.cameraPresets" />

          <button
            v-if="admin?.canEndStream"
            type="button"
            class="rounded-lg bg-red-600 py-1 font-bold text-white hover:bg-red-500"
            title="Already finished? End the stream early."
            @click="ending = true"
          >
            <i class="fas fa-power-off mr-1" aria-hidden="true"></i>End stream
          </button>
        </div>

        <div class="flex min-w-0 flex-col gap-3">
          <ul class="flex flex-wrap justify-around gap-4 rounded-lg p-2" aria-label="Audience reactions" data-testid="reactions">
            <li v-for="r in reactions" :key="r.reaction" class="flex flex-col items-center" :data-testid="`reaction-${r.reaction}`">
              <span class="text-4xl transition-opacity" :class="events.lastReaction.value === r.reaction ? 'opacity-100' : 'opacity-40'" aria-hidden="true">{{ r.reaction }}</span>
              <span class="text-1 pt-1 text-sm">{{ r.label }}</span>
            </li>
            <li v-if="!reactions.length" class="text-5 text-sm">No reactions yet.</li>
          </ul>

          <div v-if="data.chat?.enabled" class="h-[70vh] min-h-80 overflow-hidden rounded-lg border border-gray-200 dark:border-gray-800">
            <ChatPanel :key="streamId" :stream-id="streamId" />
          </div>
        </div>
      </div>
    </section>

    <EndStreamDialog v-if="ending" :course-id="courseId" :stream-id="streamId" @close="ending = false" @ended="reload" />
  </AdminLayout>
</template>
