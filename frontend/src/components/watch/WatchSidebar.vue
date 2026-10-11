<script setup lang="ts">
import { computed, ref, watch } from "vue";

import { SIDEBAR_TAB_LABELS, type SidebarTab } from "@/lib/watch";

/**
 * The tab strip and panel host of the side column.
 *
 * Every tab's panel is a named slot, `panel-<tab>` (`panel-chat`, `panel-bookmarks`,
 * `panel-playlist`, `panel-transcript`, `panel-sections`, `panel-search`). A panel is
 * mounted the first time its tab opens and then kept, hidden with `v-show`: the chat
 * must not drop its socket when the viewer glances at the playlist, and a transcript
 * should keep its scroll position. The panel fills the column and owns its own scroll.
 */
const props = defineProps<{
  tabs: SidebarTab[];
  active: SidebarTab | null;
}>();
const emit = defineEmits<{ select: [tab: SidebarTab]; close: [] }>();

const visited = ref(new Set<SidebarTab>());
watch(
  () => props.active,
  (tab) => {
    if (tab) visited.value = new Set(visited.value).add(tab);
  },
  { immediate: true },
);
const mounted = computed(() => props.tabs.filter((t) => visited.value.has(t)));

function onKey(event: KeyboardEvent, index: number): void {
  const step = event.key === "ArrowRight" ? 1 : event.key === "ArrowLeft" ? -1 : 0;
  if (!step) return;
  event.preventDefault();
  const next = props.tabs[(index + step + props.tabs.length) % props.tabs.length]!;
  emit("select", next);
  document.getElementById(`watch-tab-${next}`)?.focus();
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div class="flex items-center border-b dark:border-gray-800">
      <div role="tablist" aria-label="Side panel" class="flex min-w-0 grow gap-1 overflow-x-auto px-2">
        <button
          v-for="(tab, i) in tabs"
          :id="`watch-tab-${tab}`"
          :key="tab"
          type="button"
          role="tab"
          :aria-selected="tab === active"
          :aria-controls="`watch-panel-${tab}`"
          :tabindex="tab === active ? 0 : -1"
          class="text-4 shrink-0 border-b-2 px-3 py-2 text-sm font-medium"
          :class="tab === active ? 'border-current' : 'text-5 border-transparent hover:text-current'"
          @click="emit('select', tab)"
          @keydown="onKey($event, i)"
        >
          {{ SIDEBAR_TAB_LABELS[tab] }}
        </button>
      </div>
      <button
        type="button"
        class="tum-live-icon-button px-3 py-2"
        aria-label="Close side panel"
        title="Close"
        @click="emit('close')"
      >
        <i class="fa-solid fa-xmark" aria-hidden="true"></i>
      </button>
    </div>
    <div class="relative min-h-0 grow">
      <div
        v-for="tab in mounted"
        v-show="tab === active"
        :id="`watch-panel-${tab}`"
        :key="tab"
        role="tabpanel"
        :aria-labelledby="`watch-tab-${tab}`"
        class="h-full min-h-0"
      >
        <slot :name="`panel-${tab}`" />
      </div>
    </div>
  </div>
</template>
