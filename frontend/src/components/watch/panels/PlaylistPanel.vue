<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue";
import { RouterLink } from "vue-router";

import { thumbnailUrl } from "@/lib/streams";
import { fetchPlaylist, neighbours, type WatchPlaylistEntry } from "@/lib/watch-panels";

/**
 * The course's lectures that can be watched, the current one highlighted and scrolled
 * into view, with previous/next at the top. Entries link to /w/:slug/:id through the
 * router, so moving on does not reload the page.
 *
 * The router is the one thing this reaches for beyond its props; a lecture is a page,
 * so there is no sensible way to "emit" a navigation that is better than a link.
 */
const props = defineProps<{ streamId: number; slug: string }>();

const entries = ref<WatchPlaylistEntry[]>([]);
const loaded = ref(false);
const error = ref("");
const items = ref<HTMLElement[]>([]);

const adjacent = computed(() => neighbours(entries.value, props.streamId));
const target = (e: WatchPlaylistEntry) => `/w/${encodeURIComponent(e.slug)}/${e.streamId}`;

onMounted(async () => {
  try {
    entries.value = await fetchPlaylist(props.slug, props.streamId);
  } catch (err) {
    error.value = err instanceof Error ? err.message : "Could not load the playlist.";
  } finally {
    loaded.value = true;
  }
  await nextTick();
  const i = entries.value.findIndex((e) => e.streamId === props.streamId);
  items.value[i]?.scrollIntoView?.({ block: "center" });
});

const stepClass = "text-3 rounded-lg px-3 py-1 text-sm focus-visible:outline-2 focus-visible:outline-blue-500";
</script>

<template>
  <section class="flex h-full min-h-0 flex-col" aria-label="Playlist">
    <header class="flex items-center justify-between p-3">
      <h3 class="text-3 text-base font-bold">Playlist</h3>
      <nav class="flex" aria-label="Previous and next lecture">
        <RouterLink
          v-if="adjacent.prev"
          :to="target(adjacent.prev)"
          title="Previous"
          :class="[stepClass, 'hover:bg-gray-100 dark:hover:bg-gray-600']"
        >
          <i class="fa-solid fa-backward-step" aria-hidden="true" /><span class="sr-only">Previous lecture</span>
        </RouterLink>
        <span v-else :class="[stepClass, 'opacity-60']" aria-hidden="true"><i class="fa-solid fa-backward-step" /></span>
        <RouterLink
          v-if="adjacent.next"
          :to="target(adjacent.next)"
          title="Next"
          :class="[stepClass, 'hover:bg-gray-100 dark:hover:bg-gray-600']"
        >
          <i class="fa-solid fa-forward-step" aria-hidden="true" /><span class="sr-only">Next lecture</span>
        </RouterLink>
        <span v-else :class="[stepClass, 'opacity-60']" aria-hidden="true"><i class="fa-solid fa-forward-step" /></span>
      </nav>
    </header>

    <p v-if="error" class="px-3 text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
    <p v-else-if="loaded && !entries.length" class="text-5 px-3 text-sm">No other lectures yet.</p>

    <ul class="min-h-0 grow overflow-y-auto">
      <li v-for="(e, i) in entries" :key="e.streamId" :ref="(el) => (items[i] = el as HTMLElement)">
        <RouterLink
          :to="target(e)"
          class="flex items-center gap-3 px-3 py-2 hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-blue-500 dark:hover:bg-gray-600"
          :class="{ 'bg-gray-200 dark:bg-gray-700': e.streamId === streamId }"
          :aria-current="e.streamId === streamId ? 'page' : undefined"
        >
          <span class="relative block aspect-video h-14 shrink-0 overflow-hidden rounded bg-gray-100 dark:bg-gray-800">
            <img :src="thumbnailUrl(e.slug, e.streamId)" alt="" loading="lazy" class="h-full w-full object-cover" />
            <span v-if="e.progress > 0" class="absolute inset-x-0 bottom-0 block h-1 bg-black/30">
              <span
                class="block h-full bg-blue-500 dark:bg-indigo-500"
                :style="{ width: `${Math.round(e.progress * 100)}%` }"
                data-testid="progress"
              />
            </span>
          </span>
          <span class="text-3 min-w-0 break-words text-sm" :class="{ 'font-semibold': e.streamId === streamId }">{{
            e.name
          }}</span>
        </RouterLink>
      </li>
    </ul>
  </section>
</template>
