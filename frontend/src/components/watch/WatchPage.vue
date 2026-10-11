<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, provide, ref, shallowRef, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

import ChatPanel from "@/components/chat/ChatPanel.vue";
import WatchActionBar from "@/components/watch/actions/WatchActionBar.vue";
import WatchAdminStrip from "@/components/watch/admin/WatchAdminStrip.vue";
import WatchSidebar from "@/components/watch/panels/WatchSidebar.vue";
import SeekbarOverlay from "@/components/watch/seekbar/SeekbarOverlay.vue";
import { useSeekReporting } from "@/components/watch/seekbar/useSeekReporting";
import SplitView from "@/components/watch/split/SplitView.vue";
import WatchDescription from "@/components/watch/WatchDescription.vue";
import WatchInfo from "@/components/watch/WatchInfo.vue";
import WatchLayout from "@/components/watch/WatchLayout.vue";
import WatchMessage from "@/components/watch/WatchMessage.vue";
import WatchPlaceholder from "@/components/watch/WatchPlaceholder.vue";
import WatchPlayer from "@/components/watch/WatchPlayer.vue";
import WatchWaitingRoom from "@/components/watch/WatchWaitingRoom.vue";
import { WatchViewKind } from "@/gen/server/apiv2_pb";
import { fetchConfig } from "@/lib/config";
import { parseNumberParam } from "@/lib/player/start-time";
import {
  WatchLayout as Layout,
  layoutSegment,
  offsetsToClip,
  parseLayoutSegment,
  playbackRatesFor,
  resolveLayout,
  resolveStartTime,
  skipSecondsFor,
  spriteSource,
  streamStart,
  subtitleTracks,
  viewForLayout,
  watchDocumentTitle,
  watchPhase,
} from "@/lib/watch";
import { fetchSeekHeatmap, fetchSections, type SeekbarSection } from "@/lib/watch-actions";
import { splitTime, type WatchBookmark } from "@/lib/watch-panels";
import { createPanelPlayer } from "@/lib/watch-wiring";
import { watchContextKey, type WatchContext, type WatchController } from "@/lib/watch-context";
import { expectsRecording, pollForRecording, type RecordingPoller } from "@/lib/watch-recording-poll";
import { createWatchedMarker, markWatched } from "@/lib/watch-progress";
import { useAuthStore } from "@/stores/auth";
import { useStreamRealtime } from "@/stores/streamRealtime";
import { useWatchStore } from "@/stores/watch";
import NotFoundView from "@/views/NotFoundView.vue";

/**
 * The watch page for one stream (WatchView keys it by stream, so a navigation to
 * another lecture is a fresh page with its own chat store and player).
 *
 * ## Regions
 *
 * WatchLayout lays them out; this page fills them: the player (or the split view), the
 * lecturer's strip, title and status, the action bar, the description and the tabbed
 * side panel. Deeper descendants can reach the shared state with `useWatchContext()`
 * (lib/watch-context.ts).
 *
 * One realtime socket serves the page (`useStreamRealtime`): viewer count, the live
 * flip, title and description edits, and a rescheduled start. The chat panel listens to
 * the same one.
 */
const props = defineProps<{ slug: string; streamId: number }>();

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const store = useWatchStore();
const realtime = useStreamRealtime(props.streamId);

// ------------------------------------------------------------ loading

/** The viewer's settings pick the speeds and skip step, so the player waits for them. */
const userReady = ref(false);
const signedIn = async () => (await auth.load().catch(() => null)) !== null;

async function load(quiet = false): Promise<void> {
  await store.load(props.slug, props.streamId, { quiet, signedIn });
}

onMounted(() => {
  auth
    .load()
    .catch(() => null)
    .finally(() => (userReady.value = true));
  void load();
});

const data = computed(() => store.watch);

// One socket per tab, shared with the chat panel (it subscribes to the same store, so
// closing the panel does not close the socket the page still listens on). Subscribed
// once there is a stream the viewer may see: the server refuses the rest.
let unsubscribe: (() => void) | null = null;
let firstSync = true;
watch(
  data,
  (value) => {
    if (!value || unsubscribe) return;
    unsubscribe = realtime.subscribe({
      onEvent(event) {
        // A rescheduled lecture moves the countdown; this replaces polling the server.
        if (event.event.case === "timeChanged") store.reschedule(event.event.value.start, event.event.value.end);
      },
      // The first subscribe is a resync too, but the page has just loaded.
      onResync: () => (firstSync ? (firstSync = false) : void load(true)),
    });
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  unsubscribe?.();
  store.reset();
});

// A flip of the live state is the old page's reload: fetch the page data again, which
// brings the new playlist, or the recording, or the ended state. The server may not
// have caught up when the event arrives, so one retry follows.
const sawEnd = ref(false);
let retry: ReturnType<typeof setTimeout> | undefined;
onBeforeUnmount(() => clearTimeout(retry));
watch(
  () => realtime.live,
  async (live) => {
    const current = store.watch?.stream;
    if (live === undefined || !current || live === current.liveNow) return;
    if (!live) sawEnd.value = true;
    await load(true);
    if (store.watch?.stream?.liveNow !== live) retry = setTimeout(() => void load(true), 3000);
  },
);

// ------------------------------------------------------------ phase and layout

const phase = computed(() => (data.value ? watchPhase(data.value) : "ended"));
const requested = computed(() => parseLayoutSegment(route.params.version));
const layout = computed(() => (data.value ? resolveLayout(data.value, requested.value) : Layout.UNSPECIFIED));
const view = computed(() => (data.value ? viewForLayout(data.value, layout.value) : undefined));
const clip = computed(() => (data.value ? offsetsToClip(data.value) : undefined));
const live = computed(() => phase.value === "live");

function setLayout(next: Layout): void {
  const segment = layoutSegment(next);
  if (!segment) return;
  void router.push({ path: `/w/${props.slug}/${props.streamId}/${segment}`, query: route.query });
}

// ------------------------------------------------------------ processing recording

// A stream that just ended has no recording until the worker has transcoded it. The
// page asks again on a backoff and swaps the recording in; the player remounts through
// `playerKey` when the phase becomes "vod", so no reload is needed.
const gaveUp = ref(false);
const processing = computed(
  () => !gaveUp.value && phase.value === "ended" && !!data.value && expectsRecording(data.value, new Date(), sawEnd.value),
);
let poller: RecordingPoller | undefined;
watch(
  processing,
  (on) => {
    poller?.stop();
    poller = on
      ? pollForRecording(
          async () => {
            await load(true);
            return phase.value !== "ended";
          },
          { onGiveUp: () => (gaveUp.value = true) },
        )
      : undefined;
  },
  { immediate: true },
);
onBeforeUnmount(() => poller?.stop());

// ------------------------------------------------------------ player options

const startTime = computed(() =>
  data.value
    ? resolveStartTime(data.value, {
        t: parseNumberParam(route.query.t),
        autoSkip: auth.user?.settings.autoSkip ?? false,
      })
    : undefined,
);
const rate = computed(() => parseNumberParam(route.query.rate));
const subtitles = computed(() => (data.value ? subtitleTracks(data.value) : []));
const playbackRates = computed(() => playbackRatesFor(auth.user));
const skipSeconds = computed(() => skipSecondsFor(auth.user));

/** What remounts the player: another layout, or the stream going live/stopping. */
const playerKey = computed(() => `${layout.value}:${phase.value}`);

const controller = shallowRef<WatchController | null>(null);

// Reaching the end marks the lecture watched. Once per page view, not per player: a
// layout change remounts the player, and replaying must not report again.
const watchedMarker = createWatchedMarker(() => markWatched(props.streamId));

// ------------------------------------------------------------ live values

const title = computed(() => realtime.title ?? data.value?.stream?.displayName ?? "");
const descriptionHtml = computed(() => realtime.descriptionHtml ?? data.value?.stream?.descriptionHtml ?? "");
// The socket's count is 0 until its first event; the page data has the count from
// when it was loaded, which beats showing 0 to someone who is watching.
const viewers = computed(() => (realtime.viewers > 0 ? realtime.viewers : (data.value?.viewers ?? 0)));

// ------------------------------------------------------------ panels and seekbar

// The panels and the seek reporting keep time on the recording's clock and subscribe
// once; the controller they read changes with the layout.
const panelPlayer = createPanelPlayer(() => controller.value);
watch(controller, () => panelPlayer.rebind());
useSeekReporting(panelPlayer.player, props.streamId, { enabled: () => phase.value === "vod" });

const seekbarBookmarks = ref<{ hours: number; minutes: number; seconds: number; description: string }[]>([]);
function onBookmarks(list: WatchBookmark[]): void {
  seekbarBookmarks.value = list.map((b) => ({ ...splitTime(b.time), description: b.description }));
}
const seekbarSections = ref<SeekbarSection[]>([]);
const heatmap = ref<number[]>([]);
// Sections and the heatmap exist for recordings only; both are decoration, so a failure is silence.
watch(
  () => phase.value === "vod",
  (recording) => {
    if (!recording) return;
    fetchSections(props.slug, props.streamId).then((v) => (seekbarSections.value = v), () => undefined);
    fetchSeekHeatmap(props.streamId).then((v) => (heatmap.value = v), () => undefined);
  },
  { immediate: true },
);
/** The playable range's length, which the seekbar's fractions are of. */
const seekDuration = computed(() => {
  const total = data.value?.stream?.duration ?? 0;
  return Math.max(0, (clip.value?.end ?? total) - (clip.value?.start ?? 0));
});

const sprites = computed(() => {
  const presentation = data.value?.views.find((v) => v.kind === WatchViewKind.PRESENTATION);
  return presentation ? spriteSource(props.streamId, presentation) : undefined;
});

function setSplitController(el: unknown): void {
  if (el) controller.value = el as WatchController;
}

// ------------------------------------------------------------ document title

const previousTitle = document.title;
const brand = ref("TUM-Live");
void fetchConfig()
  .then((config) => (brand.value = config.title || brand.value))
  .catch(() => undefined);
watch(
  [data, brand, title],
  () => {
    document.title = watchDocumentTitle(data.value, brand.value);
  },
  { immediate: true },
);
onBeforeUnmount(() => (document.title = previousTitle));

// ------------------------------------------------------------ context

const sidebarTab = computed(() => store.sidebar);
const context: WatchContext = {
  watch: computed(() => data.value!),
  streamId: computed(() => props.streamId),
  slug: computed(() => props.slug),
  phase,
  layout,
  setLayout,
  clip,
  controller,
  sidebar: {
    active: sidebarTab,
    tabs: computed(() => store.tabs),
    open: store.openSidebar,
    toggle: store.toggleSidebar,
    close: store.closeSidebar,
  },
  refresh: () => load(true),
};
provide(watchContextKey, context);

</script>

<template>
  <NotFoundView v-if="store.failure === 'not-found'" />
  <WatchMessage
    v-else-if="store.failure"
    :kind="store.failure"
    @retry="load()"
  />
  <p v-else-if="!data || !userReady" class="text-5 p-6 text-sm" role="status" data-testid="watch-loading">Loading…</p>

  <WatchLayout v-else :sidebar-open="sidebarTab !== null">
    <template #player>
      <WatchWaitingRoom
        v-if="phase === 'waiting'"
        :start="streamStart(data) ?? new Date()"
        :title="title"
      />

      <div
        v-else-if="phase === 'ended'"
        class="flex aspect-video w-full flex-col items-center justify-center gap-2 bg-gray-900 p-6 text-center text-white"
        data-testid="ended"
      >
        <p class="text-xl font-semibold">This stream has ended.</p>
        <p v-if="processing" class="text-sm text-gray-300" role="status" data-testid="processing">
          <i class="fa-solid fa-spinner fa-spin mr-2" aria-hidden="true"></i>Recording is being processed. It will
          appear here as soon as it is ready.
        </p>
        <p v-else class="text-sm text-gray-300">There is no recording of it (yet).</p>
      </div>

      <SplitView
        v-else-if="layout === Layout.SPLIT"
        :key="playerKey"
        :ref="setSplitController"
        :views="data.views"
        :live="live"
        :clip="clip"
        :start-time="startTime"
        :rate="rate"
        :subtitles="subtitles"
        :sprites="sprites"
        :playback-rates="playbackRates"
        :skip-seconds="skipSeconds"
        persist
        hold-to-fast
        hotkeys="page"
        airplay
      />

      <WatchPlayer
        v-else-if="view"
        :key="playerKey"
        :stream-id="streamId"
        :view="view"
        :live="live"
        :subtitles="subtitles"
        :clip="clip"
        :start-time="startTime"
        :rate="rate"
        :playback-rates="playbackRates"
        :skip-seconds="skipSeconds"
        :silences="data.silences"
        :report-progress="phase === 'vod' && data.viewer?.signedIn === true"
        :recording-duration="data.stream?.duration"
        @controller="controller = $event"
        @ended="watchedMarker.fire()"
      >
        <template #seekbar="{ container }">
          <SeekbarOverlay
            v-if="phase === 'vod'"
            :container="container"
            :duration="seekDuration"
            :offset="clip?.start ?? 0"
            :sections="seekbarSections"
            :bookmarks="seekbarBookmarks"
            :heatmap="heatmap"
          />
        </template>
      </WatchPlayer>

      <WatchPlaceholder v-else label="This stream has no playable view." />
    </template>

    <template v-if="data.admin && data.course" #admin-strip>
      <WatchAdminStrip
        :admin="data.admin"
        :course-id="data.course.id"
        :stream-id="streamId"
        :slug="slug"
        :live-now="data.stream?.liveNow === true"
        @ended="load(true)"
      />
    </template>

    <template #info>
      <div class="flex items-start justify-between gap-3">
        <WatchInfo :watch="data" :phase="phase" :title="title" :viewers="viewers" />
        <button
          v-if="sidebarTab === null && store.tabs.length > 0"
          type="button"
          class="tum-live-button tum-live-button-tertiary shrink-0 text-xs"
          data-testid="open-sidebar"
          @click="store.openSidebar(store.tabs[0]!)"
        >
          <i class="fa-solid fa-table-columns mr-2" aria-hidden="true"></i>Panels
        </button>
      </div>
    </template>

    <template v-if="phase !== 'waiting' && data.viewer" #action-bar>
      <WatchActionBar
        :stream-id="streamId"
        :views="data.views"
        :split-available="data.splitAvailable"
        :attachments="data.attachments"
        :downloads="data.downloads"
        :viewer="data.viewer"
        :layout="layout"
        :live="live"
        :controller="controller ?? undefined"
        :user-name="auth.user?.name"
        @select-layout="setLayout($event.layout)"
      />
    </template>

    <template #description>
      <WatchDescription :html="descriptionHtml" />
    </template>

    <template #sidebar>
      <div class="flex h-full min-h-0 flex-col">
        <div class="flex justify-end border-b px-2 py-1 dark:border-gray-800">
          <button
            type="button"
            class="tum-live-icon-button px-2 py-1 text-sm"
            aria-label="Close side panel"
            title="Close"
            data-testid="close-sidebar"
            @click="store.closeSidebar()"
          >
            <i class="fa-solid fa-xmark" aria-hidden="true"></i>
          </button>
        </div>
        <div class="min-h-0 grow">
          <WatchSidebar
            :stream-id="streamId"
            :slug="slug"
            :player="panelPlayer.player"
            :subtitle-languages="data.stream?.recording ? data.subtitleLanguages : []"
            :show-chat="data.chat?.enabled === true"
            :show-bookmarks="data.viewer?.signedIn === true"
            :show-playlist="data.viewer?.signedIn === true"
            :show-sections="data.stream?.recording === true"
            :show-search="data.stream?.recording === true && data.subtitleLanguages.length > 0"
            @bookmarks="onBookmarks"
          >
            <template #chat>
              <ChatPanel :stream-id="streamId" />
            </template>
          </WatchSidebar>
        </div>
      </div>
    </template>
  </WatchLayout>
</template>
