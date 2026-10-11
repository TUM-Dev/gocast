<script setup lang="ts">
import { computed, ref, watch } from "vue";

import { formatClock, parseShareTime, shareUrl } from "@/lib/watch-actions";
import ActionDialog from "./ActionDialog.vue";

/**
 * The page's link, with an optional "start at the current time" (`?t=`). `pageUrl` is
 * the address to share (the shell passes `location.href`; the query is dropped here so
 * `?rate=` and friends are not shared). `currentTime` is sampled when the dialog opens,
 * not live; the viewer can then edit it (hh:mm:ss), and the URL follows. Not offered for
 * a live stream (`live`): there is no time to return to.
 */
const props = defineProps<{
  open: boolean;
  pageUrl: string;
  live?: boolean;
  /** Where the viewer is, in recording seconds. */
  currentTime: () => number;
  /** The length of what can be played, in the same seconds; edits are clamped to it. */
  duration?: () => number;
}>();
const emit = defineEmits<{ close: [] }>();

const withTime = ref(false);
const seconds = ref(0);
/** What the field shows; differs from `seconds` while the viewer is typing. */
const text = ref("00:00:00");
const invalid = ref(false);
const copied = ref(false);
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

watch(
  () => props.open,
  (open) => {
    if (open) {
      withTime.value = false;
      seconds.value = clampSeconds(Math.floor(props.currentTime()));
      text.value = formatClock(seconds.value);
      invalid.value = false;
      copied.value = false;
    }
  },
);

function clampSeconds(value: number): number {
  const length = props.duration?.();
  const upper = length !== undefined && Number.isFinite(length) ? Math.floor(length) : Infinity;
  return Math.min(Math.max(0, value), upper);
}

// An unparseable entry keeps the last good time in the URL and marks the field; a
// parseable one past the end is clamped when the field loses focus (while typing, "1"
// on the way to "1:30:00" must not jump).
function onInput(): void {
  const parsed = parseShareTime(text.value);
  invalid.value = parsed === null;
  if (parsed !== null) seconds.value = clampSeconds(parsed);
}
function onBlur(): void {
  if (!invalid.value) text.value = formatClock(seconds.value);
}

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
      <label for="watch-share-time" class="text-sm font-light">Start at</label>
      <input
        v-if="withTime"
        id="watch-share-time-value"
        v-model="text"
        type="text"
        inputmode="numeric"
        autocomplete="off"
        placeholder="hh:mm:ss"
        aria-label="Start time (hh:mm:ss)"
        :aria-invalid="invalid"
        class="tum-live-input text-5 h-10 w-28 text-center text-sm"
        :class="invalid ? 'outline-2 outline-red-500' : ''"
        @input="onInput"
        @blur="onBlur"
      />
      <p v-if="withTime && invalid" class="w-full text-xs text-red-600 dark:text-red-400" role="alert">
        Enter the time as hh:mm:ss.
      </p>
    </div>
  </ActionDialog>
</template>
