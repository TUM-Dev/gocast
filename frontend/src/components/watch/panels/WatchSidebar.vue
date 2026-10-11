<script setup lang="ts">
import { computed, ref, watch } from "vue";

import BookmarksPanel from "@/components/watch/panels/BookmarksPanel.vue";
import PlaylistPanel from "@/components/watch/panels/PlaylistPanel.vue";
import SectionsPanel from "@/components/watch/panels/SectionsPanel.vue";
import SubtitleSearch from "@/components/watch/panels/SubtitleSearch.vue";
import TranscriptPanel from "@/components/watch/panels/TranscriptPanel.vue";
import type { PanelPlayer, WatchBookmark, WatchSection } from "@/lib/watch-panels";
import { SIDEBAR_TABS, readSidebarTab, writeSidebarTab, type SidebarTab } from "@/lib/watch-panels-tabs";

/**
 * The watch page's sidebar: a tab per panel. The chat is not built here; the shell
 * puts its ChatPanel in the `chat` slot, so this stays free of the chat store.
 *
 * Which tabs exist is the shell's call, by the `show*` props (no subtitles: no
 * transcript; signed out: no bookmarks; chat off: no chat). A tab that is hidden is
 * not mounted at all. The chat and the bookmarks mount straight away (the chat must
 * stay connected, the bookmarks feed the seekbar); the rest mount on first visit and
 * are then kept, so switching back neither refetches nor loses the scroll position.
 *
 * The chosen tab is remembered across visits.
 */
const props = withDefaults(
  defineProps<{
    streamId: number;
    slug: string;
    player: PanelPlayer;
    /** Subtitle languages of the stream; none hides the transcript. */
    subtitleLanguages?: string[];
    showChat?: boolean;
    showBookmarks?: boolean;
    showSections?: boolean;
    showPlaylist?: boolean;
    showSearch?: boolean;
  }>(),
  {
    subtitleLanguages: () => [],
    showChat: true,
    showBookmarks: true,
    showSections: true,
    showPlaylist: true,
    showSearch: true,
  },
);

const emit = defineEmits<{
  /** The viewer's bookmarks, whenever they change; for seekbar markers. */
  bookmarks: [bookmarks: WatchBookmark[]];
  /** The lecture's sections once loaded; for seekbar chapters. */
  sections: [sections: WatchSection[]];
  /** The tab now showing. */
  tab: [tab: SidebarTab];
}>();

const visible = computed<SidebarTab[]>(() => {
  const on: Record<SidebarTab, boolean> = {
    chat: props.showChat,
    bookmarks: props.showBookmarks,
    sections: props.showSections,
    playlist: props.showPlaylist,
    transcript: props.subtitleLanguages.length > 0,
    search: props.showSearch,
  };
  return SIDEBAR_TABS.filter((t) => on[t.id]).map((t) => t.id);
});

const selected = ref<SidebarTab | null>(readSidebarTab());
const current = computed<SidebarTab | null>(() =>
  selected.value && visible.value.includes(selected.value) ? selected.value : (visible.value[0] ?? null),
);

// Which tabs have been opened, so lazy panels mount once and stay.
const opened = ref(new Set<SidebarTab>());
watch(
  current,
  (tab) => {
    if (!tab) return;
    opened.value.add(tab);
    emit("tab", tab);
  },
  { immediate: true },
);

function select(tab: SidebarTab): void {
  selected.value = tab;
  writeSidebarTab(tab);
}

const tabButtons = ref<HTMLElement[]>([]);
function onKey(e: KeyboardEvent): void {
  const step = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
  if (!step || !current.value) return;
  const list = visible.value;
  const next = list[(list.indexOf(current.value) + step + list.length) % list.length];
  select(next);
  tabButtons.value[list.indexOf(next)]?.focus();
}

const labels = Object.fromEntries(SIDEBAR_TABS.map((t) => [t.id, t.label]));
const mounted = (tab: SidebarTab) => opened.value.has(tab) || tab === "chat" || tab === "bookmarks";
</script>

<template>
  <aside class="flex h-full min-h-0 flex-col" aria-label="Lecture sidebar">
    <div class="flex overflow-x-auto border-b dark:border-gray-800" role="tablist" aria-label="Sidebar" @keydown="onKey">
      <button
        v-for="(tab, i) in visible"
        :id="`watch-tab-${tab}`"
        :key="tab"
        :ref="(el) => (tabButtons[i] = el as HTMLElement)"
        type="button"
        role="tab"
        class="text-3 shrink-0 border-b-2 px-3 py-2 text-sm focus-visible:outline-2 focus-visible:outline-blue-500"
        :class="tab === current ? 'border-blue-500 font-semibold dark:border-indigo-500' : 'border-transparent'"
        :aria-selected="tab === current"
        :aria-controls="`watch-panel-${tab}`"
        :tabindex="tab === current ? 0 : -1"
        @click="select(tab)"
      >
        {{ labels[tab] }}
      </button>
    </div>

    <div class="min-h-0 grow">
      <template v-for="tab in visible" :key="tab">
        <div
          v-if="mounted(tab)"
          v-show="tab === current"
          :id="`watch-panel-${tab}`"
          role="tabpanel"
          :aria-labelledby="`watch-tab-${tab}`"
          class="h-full"
        >
          <slot v-if="tab === 'chat'" name="chat" />
          <BookmarksPanel
            v-else-if="tab === 'bookmarks'"
            :stream-id="streamId"
            :player="player"
            @bookmarks="emit('bookmarks', $event)"
          />
          <SectionsPanel
            v-else-if="tab === 'sections'"
            :stream-id="streamId"
            :slug="slug"
            :player="player"
            @sections="emit('sections', $event)"
          />
          <PlaylistPanel v-else-if="tab === 'playlist'" :stream-id="streamId" :slug="slug" />
          <TranscriptPanel
            v-else-if="tab === 'transcript'"
            :stream-id="streamId"
            :slug="slug"
            :languages="subtitleLanguages"
            :player="player"
          />
          <SubtitleSearch v-else-if="tab === 'search'" :stream-id="streamId" :player="player" />
        </div>
      </template>
    </div>
  </aside>
</template>
