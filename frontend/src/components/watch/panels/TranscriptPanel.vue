<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";

import {
  activeCueIndex,
  fetchTranscript,
  saveTextFile,
  transcriptText,
  type TranscriptCue,
} from "@/lib/watch-panels-transcript";
import { formatTime, type PanelPlayer } from "@/lib/watch-panels";

/**
 * The recording's subtitles as readable text, beta. Loads the chosen language, marks
 * the cue being spoken and follows it, click jumps, "Export transcript" saves the
 * text. `languages` are the subtitle languages the stream has (from getStreamWatch);
 * the sidebar hides this panel when there are none.
 */
const props = defineProps<{ streamId: number; slug: string; languages: string[]; player: PanelPlayer }>();

const lang = ref(props.languages[0] ?? "");
const cues = ref<TranscriptCue[]>([]);
const active = ref(-1);
const loading = ref(false);
const error = ref("");
const follow = ref(true);
const items = ref<HTMLElement[]>([]);

let off: (() => void) | undefined;
let request = 0;

async function load(): Promise<void> {
  if (!lang.value) return;
  const mine = ++request;
  loading.value = true;
  error.value = "";
  try {
    const loaded = await fetchTranscript(props.slug, props.streamId, lang.value);
    // A slower answer for a language the viewer has already left must not win.
    if (mine !== request) return;
    cues.value = loaded;
    sync();
  } catch (err) {
    if (mine !== request) return;
    cues.value = [];
    error.value = err instanceof Error ? err.message : "Could not load the transcript.";
  } finally {
    if (mine === request) loading.value = false;
  }
}

function sync(): void {
  active.value = activeCueIndex(cues.value, props.player.getCurrentTime());
}

onMounted(() => {
  off = props.player.on("timeupdate", sync);
  void load();
});
onBeforeUnmount(() => off?.());
watch(lang, load);

watch(active, async (i) => {
  if (!follow.value || i < 0) return;
  await nextTick();
  // "nearest", not "center": centring scrolls the whole page along with the list.
  items.value[i]?.scrollIntoView?.({ block: "nearest", behavior: "smooth" });
});

function exportText(): void {
  saveTextFile(transcriptText(cues.value), "transcript.txt");
}
</script>

<template>
  <section class="flex h-full min-h-0 flex-col" aria-label="Transcript">
    <header class="flex flex-wrap items-center justify-between gap-2 p-3">
      <div class="flex items-center gap-2">
        <h3 class="text-3 text-base font-bold">Transcript</h3>
        <span class="rounded-full bg-red-500 px-2 py-0.5 text-xs font-semibold italic text-white">beta</span>
      </div>
      <div class="flex items-center gap-2">
        <label v-if="languages.length > 1" class="text-sm">
          <span class="sr-only">Language</span>
          <select v-model="lang" class="text-3 rounded bg-gray-50 px-2 py-1 text-sm dark:bg-gray-600">
            <option v-for="l in languages" :key="l" :value="l">{{ l }}</option>
          </select>
        </label>
        <button
          type="button"
          class="text-3 rounded-full px-3 py-1 text-sm hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-blue-500 dark:hover:bg-gray-600"
          :class="{ 'bg-gray-100 font-semibold dark:bg-gray-700': follow }"
          :aria-pressed="follow"
          @click="follow = !follow"
        >
          Follow playback
        </button>
        <button
          type="button"
          class="text-3 rounded-full px-3 py-1 text-sm font-semibold hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-blue-500 disabled:opacity-50 dark:hover:bg-gray-600"
          :disabled="!cues.length"
          @click="exportText"
        >
          Export transcript
        </button>
      </div>
    </header>

    <p v-if="error" class="px-3 text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
    <p v-else-if="loading" class="text-5 px-3 text-sm">Loading...</p>
    <p v-else-if="!cues.length" class="text-5 px-3 text-sm">There is no transcript for this language.</p>

    <ul class="min-h-0 grow overflow-y-auto">
      <li v-for="(c, i) in cues" :key="i" :ref="(el) => (items[i] = el as HTMLElement)">
        <button
          type="button"
          class="flex w-full items-start gap-2 rounded px-3 py-2 text-left hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-blue-500 dark:hover:bg-gray-700"
          :class="{ 'bg-blue-100 dark:bg-blue-700': i === active }"
          :aria-current="i === active ? 'true' : undefined"
          @click="player.seek(c.start)"
        >
          <span class="text-5 mt-0.5 shrink-0 text-xs tabular-nums">{{ formatTime(c.start) }}</span>
          <span class="text-3 text-sm">{{ c.text }}</span>
        </button>
      </li>
    </ul>
  </section>
</template>
