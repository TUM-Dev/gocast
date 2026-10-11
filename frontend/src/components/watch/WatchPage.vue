<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, provide, ref, shallowRef, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

import ChatPanel from "@/components/chat/ChatPanel.vue";
import WatchDescription from "@/components/watch/WatchDescription.vue";
import WatchInfo from "@/components/watch/WatchInfo.vue";
import WatchLayout from "@/components/watch/WatchLayout.vue";
import WatchMessage from "@/components/watch/WatchMessage.vue";
import WatchPlaceholder from "@/components/watch/WatchPlaceholder.vue";
import WatchPlayer from "@/components/watch/WatchPlayer.vue";
import WatchSidebar from "@/components/watch/WatchSidebar.vue";
import WatchWaitingRoom from "@/components/watch/WatchWaitingRoom.vue";
import { fetchConfig } from "@/lib/config";
import { parseNumberParam } from "@/lib/player/start-time";
import {
  WatchLayout as Layout,
  availableLayouts,
  countdown,
  layoutSegment,
  offsetsToClip,
  parseLayoutSegment,
  playbackRatesFor,
  resolveLayout,
  resolveStartTime,
  skipSecondsFor,
  streamStart,
  subtitleTracks,
  viewForLayout,
  waitingPollInterval,
  watchDocumentTitle,
  watchPhase,
  type SidebarTab,
} from "@/lib/watch";
import { watchContextKey, type WatchContext, type WatchController } from "@/lib/watch-context";
import { useAuthStore } from "@/stores/auth";
import { useChatStore } from "@/stores/chat";
import { useWatchStore } from "@/stores/watch";
import NotFoundView from "@/views/NotFoundView.vue";

/**
 * The watch page for one stream (WatchView keys it by stream, so a navigation to
 * another lecture is a fresh page with its own chat store and player).
 *
 * ## Region contract
 *
 * The regions other parts of the watch page plug into. Each is a spot below marked
 * `TODO(integrator)`; the component named there replaces the placeholder. All of them
 * can also reach the shared state with `useWatchContext()` (lib/watch-context.ts).
 *
 *  - action bar  (`<template #action-bar>`): needs `watch`, `layout`, `layouts`,
 *    `setLayout`, `controller`, `sidebar` (open/toggle a tab).
 *  - seekbar overlays (`<template #seekbar>` of WatchPlayer): `controller` and `clip`.
 *  - admin strip (`<template #admin-strip>`, only when `watch.admin`): `watch`.
 *  - sidebar panels (`<template #panel-<tab>>`): `watch`, `streamId`, `slug`,
 *    `controller`, and `active` for panels that must pause work while hidden.
 *  - split view  (the player region when `layout === SPLIT`): `watch`, `clip`, start
 *    time and rate; it builds its own two players.
 */
const props = defineProps<{ slug: string; streamId: number }>();

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const store = useWatchStore();
const chat = useChatStore(props.streamId);

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

// The chat store doubles as the page's realtime socket (viewers, live flips, title,
// description) until the shared stream socket replaces it. Started once there is a
// stream the viewer may see; ChatPanel's own start is then a no-op.
watch(
  data,
  (value) => {
    if (value) chat.start();
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  chat.stop();
  store.reset();
});

// A flip of the live state is the old page's reload: fetch the page data again, which
// brings the new playlist, or the recording, or the ended state. The server may not
// have caught up when the event arrives, so one retry follows.
let retry: ReturnType<typeof setTimeout> | undefined;
watch(
  () => chat.live,
  async (live) => {
    const current = store.watch?.stream;
    if (live === undefined || !current || live === current.liveNow) return;
    await load(true);
    if (store.watch?.stream?.liveNow !== live) retry = setTimeout(() => void load(true), 3000);
  },
);

// ------------------------------------------------------------ phase and layout

const phase = computed(() => (data.value ? watchPhase(data.value) : "ended"));
const requested = computed(() => parseLayoutSegment(route.params.version));
const layout = computed(() => (data.value ? resolveLayout(data.value, requested.value) : Layout.UNSPECIFIED));
const layouts = computed(() => (data.value ? availableLayouts(data.value) : []));
const view = computed(() => (data.value ? viewForLayout(data.value, layout.value) : undefined));
const clip = computed(() => (data.value ? offsetsToClip(data.value) : undefined));
const live = computed(() => phase.value === "live");

function setLayout(next: Layout): void {
  const segment = layoutSegment(next);
  if (!segment) return;
  void router.push({ path: `/w/${props.slug}/${props.streamId}/${segment}`, query: route.query });
}

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

// ------------------------------------------------------------ live values

const title = computed(() => chat.title ?? data.value?.stream?.displayName ?? "");
const descriptionHtml = computed(() => chat.descriptionHtml ?? data.value?.stream?.descriptionHtml ?? "");
// The socket's count is 0 until its first event; the page data has the count from
// when it was loaded, which beats showing 0 to someone who is watching.
const viewers = computed(() => (chat.viewers > 0 ? chat.viewers : (data.value?.viewers ?? 0)));

// ------------------------------------------------------------ waiting room poll

// StreamTimeChangedEvent is not on the chat store yet, so a rescheduled lecture is
// noticed by asking again: lightly while waiting, faster once the start has passed.
let poll: ReturnType<typeof setTimeout> | undefined;
function stopPolling(): void {
  clearTimeout(poll);
  poll = undefined;
}
function schedulePoll(): void {
  const start = data.value ? streamStart(data.value) : undefined;
  const wait = start ? waitingPollInterval(countdown(start, new Date())) : 30_000;
  poll = setTimeout(async () => {
    await load(true);
    if (phase.value === "waiting") schedulePoll();
  }, wait);
}
watch(
  phase,
  (now) => {
    stopPolling();
    if (now === "waiting" && data.value) schedulePoll();
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  stopPolling();
  clearTimeout(retry);
});

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

function onSelectTab(tab: SidebarTab): void {
  store.openSidebar(tab);
}
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
        <p class="text-sm text-gray-300">There is no recording of it (yet).</p>
      </div>

      <template v-else-if="layout === Layout.SPLIT">
        <!-- TODO(integrator): extras agent's split view replaces this placeholder:
             <WatchSplitView :watch="data" :clip="clip" :start-time="startTime" :rate="rate" /> -->
        <WatchPlaceholder label="Split view is not wired in yet." />
      </template>

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
      >
        <template #seekbar="{ controller: c, clip: cl }">
          <!-- TODO(integrator): actions agent's seekbar overlays (section dividers,
               bookmark markers, heatmap): <... :controller="c" :clip="cl" /> -->
          <span v-if="false">{{ c }}{{ cl }}</span>
        </template>
      </WatchPlayer>

      <WatchPlaceholder v-else label="This stream has no playable view." />
    </template>

    <template v-if="data.admin" #admin-strip>
      <!-- TODO(integrator): extras agent's admin strip: <... :watch="data" /> -->
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

    <template #action-bar>
      <!-- TODO(integrator): actions agent's action bar:
           <... :watch="data" :layout="layout" :layouts="layouts" :set-layout="setLayout"
                :controller="controller" :sidebar="context.sidebar" /> -->
      <span v-if="false">{{ layouts }}</span>
    </template>

    <template #description>
      <WatchDescription :html="descriptionHtml" />
    </template>

    <template #sidebar>
      <WatchSidebar :tabs="store.tabs" :active="sidebarTab" @select="onSelectTab" @close="store.closeSidebar()">
        <template #panel-chat>
          <ChatPanel :stream-id="streamId" />
        </template>
        <template #panel-bookmarks>
          <!-- TODO(integrator): panels agent: <... :stream-id :controller /> -->
          <WatchPlaceholder label="Bookmarks are not wired in yet." />
        </template>
        <template #panel-playlist>
          <!-- TODO(integrator): panels agent -->
          <WatchPlaceholder label="The playlist is not wired in yet." />
        </template>
        <template #panel-sections>
          <!-- TODO(integrator): panels agent -->
          <WatchPlaceholder label="Sections are not wired in yet." />
        </template>
        <template #panel-transcript>
          <!-- TODO(integrator): panels agent -->
          <WatchPlaceholder label="The transcript is not wired in yet." />
        </template>
        <template #panel-search>
          <!-- TODO(integrator): panels agent -->
          <WatchPlaceholder label="Search is not wired in yet." />
        </template>
      </WatchSidebar>
    </template>
  </WatchLayout>
</template>
