<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue";

import {
  fetchTranscodingProgress,
  pollTranscodingProgress,
  shouldPollProgress,
  type TranscodingProgress,
} from "@/lib/lecture-content";

/**
 * How far each version of a converting lecture has got, kept current by polling
 * while anything is still transcoding. When nothing is, `done` asks for a reload, so
 * the card shows the finished recording. A version whose conversion failed stays, with
 * the reason, until the next upload of it replaces it.
 */
const props = defineProps<{
  courseId: number;
  lectureId: number;
  converting: boolean;
  progresses: TranscodingProgress[];
}>();

const emit = defineEmits<{ done: [] }>();

const shown = ref<TranscodingProgress[]>(props.progresses);
let stop: (() => void) | null = null;

function start(): void {
  if (stop || !shouldPollProgress(props.converting, props.progresses)) return;
  stop = pollTranscodingProgress({
    fetch: () => fetchTranscodingProgress(props.courseId, props.lectureId),
    onProgress: (p) => {
      shown.value = p;
    },
    onDone: (failed) => {
      stop = null;
      shown.value = failed;
      emit("done");
    },
  });
}

// A reload hands in fresh values; it may also be what tells us a new version started.
watch(
  () => [props.converting, props.progresses] as const,
  () => {
    shown.value = props.progresses;
    if (!shouldPollProgress(props.converting, props.progresses)) {
      stop?.();
      stop = null;
    }
    start();
  },
  { immediate: true },
);

onBeforeUnmount(() => stop?.());
</script>

<template>
  <div v-if="shown.length" class="mt-1 flex flex-col gap-1" aria-label="Transcoding progress">
    <div v-for="p in shown" :key="p.version" class="text-xs">
      <p v-if="p.failed" class="font-semibold text-red-700 dark:text-red-400" role="alert">
        <i class="fas fa-triangle-exclamation mr-1"></i>{{ p.version }}: conversion failed.
        <span v-if="p.error" class="font-normal">{{ p.error }}</span>
        <span class="font-normal"> Upload the video again to retry.</span>
      </p>
      <template v-else>
        <span class="text-4 font-semibold">{{ p.version }} ({{ p.progress }}%)</span>
        <div
          class="h-1.5 w-full rounded-full bg-gray-200 dark:bg-gray-700"
          role="progressbar"
          :aria-label="`${p.version} transcoding`"
          :aria-valuenow="p.progress"
          aria-valuemin="0"
          aria-valuemax="100"
        >
          <div class="h-1.5 rounded-full bg-blue-600 dark:bg-blue-500" :style="{ width: `${p.progress}%` }"></div>
        </div>
      </template>
    </div>
  </div>
</template>
