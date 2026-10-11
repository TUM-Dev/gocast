<script setup lang="ts">
import { computed, ref, watch } from "vue";

import { formatClock, shareUrl } from "@/lib/watch-actions";
import ActionDialog from "./ActionDialog.vue";

/**
 * The page's link, with an optional "start at the current time" (`?t=`). `pageUrl` is
 * the address to share (the shell passes `location.href`; the query is dropped here so
 * `?rate=` and friends are not shared). `currentTime` is sampled when the dialog opens
 * and by the checkbox, not live. Not offered for a live stream (`live`): there is no
 * time to return to.
 */
const props = defineProps<{
  open: boolean;
  pageUrl: string;
  live?: boolean;
  /** Where the viewer is, in recording seconds. */
  currentTime: () => number;
}>();
const emit = defineEmits<{ close: [] }>();

const withTime = ref(false);
const seconds = ref(0);
const copied = ref(false);
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

watch(
  () => props.open,
  (open) => {
    if (open) {
      withTime.value = false;
      seconds.value = Math.floor(props.currentTime());
      copied.value = false;
    }
  },
);

const url = computed(() => shareUrl(props.pageUrl, withTime.value ? seconds.value : undefined));

async function copy(): Promise<void> {
  try {
    await navigator.clipboard.writeText(url.value);
    copied.value = true;
    clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => (copied.value = false), 1500);
  } catch {
    // Clipboard access can be denied; the URL stays selectable in the field.
  }
}
</script>

<template>
  <ActionDialog :open="open" title="Share" @close="emit('close')">
    <label class="text-4 mb-1 block text-xs font-light" for="watch-share-url">URL</label>
    <div class="flex flex-col items-stretch gap-2 sm:flex-row sm:items-center">
      <input
        id="watch-share-url"
        type="text"
        readonly
        :value="url"
        class="tum-live-input text-5 h-10 min-w-0 grow truncate text-sm"
        @focus="($event.target as HTMLInputElement).select()"
      />
      <button
        type="button"
        class="whitespace-nowrap rounded-full bg-blue-500 px-4 py-2 text-xs text-white hover:bg-blue-600 dark:bg-indigo-600 dark:hover:bg-indigo-700"
        @click="copy"
      >
        <i class="fa-solid mr-2" :class="copied ? 'fa-check' : 'fa-clipboard'"></i>
        <span role="status">{{ copied ? "Copied" : "Copy" }}</span>
      </button>
    </div>
    <div v-if="!live" class="flex flex-wrap items-center gap-2 pt-3">
      <input id="watch-share-time" v-model="withTime" type="checkbox" class="h-5 w-5" />
      <label for="watch-share-time" class="text-sm font-light">Start at {{ formatClock(seconds) }}</label>
    </div>
  </ActionDialog>
</template>
