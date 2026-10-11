<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue";

import {
  MIN_SEARCH_LENGTH,
  SearchUnavailableError,
  formatTime,
  searchSubtitles,
  type PanelPlayer,
  type SubtitleHit,
} from "@/lib/watch-panels";

/**
 * Search in what the lecturer said: type, see the matching lines with the lines
 * around them, click to jump. Searches as the viewer pauses typing, not on every key,
 * and ignores answers to queries they have already typed past.
 *
 * Search runs on its own service that may be down (503): that is reported as "not
 * available" and clears itself on the next query, rather than looking like "no hits".
 */
const props = defineProps<{ streamId: number; player: PanelPlayer; debounceMs?: number }>();

const query = ref("");
const hits = ref<SubtitleHit[]>([]);
const searching = ref(false);
const searched = ref(false);
const unavailable = ref(false);
const error = ref("");

let timer: ReturnType<typeof setTimeout> | undefined;
let request = 0;

async function run(q: string): Promise<void> {
  const mine = ++request;
  searching.value = true;
  try {
    const found = await searchSubtitles(props.streamId, q);
    if (mine !== request) return;
    hits.value = found;
    unavailable.value = false;
    error.value = "";
    searched.value = true;
  } catch (err) {
    if (mine !== request) return;
    hits.value = [];
    searched.value = false;
    unavailable.value = err instanceof SearchUnavailableError;
    error.value = unavailable.value ? "" : err instanceof Error ? err.message : "Search failed.";
  } finally {
    if (mine === request) searching.value = false;
  }
}

watch(query, (value) => {
  clearTimeout(timer);
  const q = value.trim();
  if (q.length < MIN_SEARCH_LENGTH) {
    request++; // drop whatever is still on its way
    hits.value = [];
    searching.value = false;
    searched.value = false;
    unavailable.value = false;
    error.value = "";
    return;
  }
  timer = setTimeout(() => void run(q), props.debounceMs ?? 300);
});
onBeforeUnmount(() => clearTimeout(timer));
</script>

<template>
  <section class="flex h-full min-h-0 flex-col" aria-label="Search in lecture">
    <header class="p-3">
      <label class="block">
        <span class="sr-only">Search in lecture</span>
        <input
          v-model="query"
          type="search"
          placeholder="Search in lecture"
          autocomplete="off"
          class="text-2 w-full rounded-lg border border-gray-600 bg-transparent p-2 font-light"
        />
      </label>
    </header>

    <p v-if="unavailable" class="text-5 px-3 text-sm" role="status">
      Search is not available right now. Try again later.
    </p>
    <p v-else-if="error" class="px-3 text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
    <p v-else-if="searching" class="text-5 px-3 text-sm" role="status">Searching...</p>
    <p v-else-if="searched && !hits.length" class="text-5 px-3 text-sm" role="status">Nothing found.</p>

    <ul class="min-h-0 grow overflow-y-auto px-3">
      <li v-for="(h, i) in hits" :key="`${h.time}-${i}`" class="my-2">
        <button
          type="button"
          class="text-3 flex w-full gap-3 rounded bg-white p-2 text-left hover:outline focus-visible:outline-2 focus-visible:outline-blue-500 dark:bg-gray-800 dark:hover:bg-gray-700"
          @click="player.seek(h.time)"
        >
          <span class="my-auto shrink-0 font-semibold tabular-nums">{{ formatTime(h.time) }}</span>
          <span class="min-w-0">
            <span class="block text-xs">{{ h.before }}</span>
            <span class="block text-sm font-bold">{{ h.text }}</span>
            <span class="block text-xs">{{ h.after }}</span>
          </span>
        </button>
      </li>
    </ul>
  </section>
</template>
