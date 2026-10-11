<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue";

import type { Clip } from "@/lib/player/clip";
import { silenceAt, toClipTime, type WatchSilence } from "@/lib/watch";
import type { WatchController } from "@/lib/watch-context";

/**
 * "Skip silence": offered while the playhead is inside a stretch without speech, and
 * jumps to its end. Silences are on the recording's clock; the controller's is the
 * clip's, hence the conversion both ways.
 */
const props = defineProps<{
  controller: WatchController | null;
  silences: readonly WatchSilence[];
  clip?: Clip;
}>();

const target = ref<number | null>(null);
let unsubscribe: (() => void)[] = [];

function check(): void {
  const c = props.controller;
  if (!c) return;
  const silence = silenceAt(props.silences, c.toRecordingTime(c.getCurrentTime()));
  target.value = silence ? toClipTime(silence.end, props.clip) : null;
}

watch(
  () => props.controller,
  (c) => {
    unsubscribe.forEach((u) => u());
    unsubscribe = [];
    target.value = null;
    if (!c) return;
    // timeupdate is about 4 Hz, which is as fine as a button appearing needs to be;
    // seeked covers a seek while paused.
    unsubscribe = [c.on("timeupdate", check), c.on("seeked", check)];
  },
  { immediate: true },
);
onBeforeUnmount(() => unsubscribe.forEach((u) => u()));

function skip(): void {
  if (target.value !== null) props.controller?.seek(target.value);
  target.value = null;
}
</script>

<template>
  <button
    v-if="target !== null"
    type="button"
    class="absolute right-3 bottom-16 z-10 rounded-lg bg-black/70 px-3 py-2 text-sm font-medium text-white hover:bg-black/90 focus-visible:outline-2 focus-visible:outline-white"
    data-testid="skip-silence"
    @click="skip"
  >
    Skip silence <i class="fa-solid fa-forward-step ml-1" aria-hidden="true"></i>
  </button>
</template>
