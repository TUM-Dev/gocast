<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";

import { opfsSupported, type RecordingMeta } from "@/lib/recorder/chunk-store";
import { discardRecording, resumeRecording, saveLocalCopy, unfinishedRecordings } from "@/lib/recorder/recovery";
import type { QueueStatus } from "@/lib/recorder/upload-queue";
import { useBrowserRecordingStore } from "@/stores/browser-recording";

/**
 * Recordings of the course this browser still holds because the tab recording them
 * was closed, crashed or reloaded before they were uploaded. They can be uploaded and
 * finished from here, saved as files, or thrown away.
 */
const props = defineProps<{ courseId: number }>();
const emit = defineEmits<{ finished: [] }>();

const rec = useBrowserRecordingStore();
const found = ref<RecordingMeta[]>([]);
const status = reactive<Record<string, QueueStatus>>({});
const error = ref("");

const LABEL = { PRES: "screen", CAM: "camera" } as const;

async function load(): Promise<void> {
  if (!opfsSupported()) return;
  try {
    found.value = await unfinishedRecordings(rec.chunks, props.courseId);
  } catch {
    found.value = [];
  }
}

onMounted(load);

async function finish(meta: RecordingMeta): Promise<void> {
  error.value = "";
  const queue = await resumeRecording(rec.chunks, meta, (s) => {
    status[meta.key] = s;
    if (s.state === "done") {
      found.value = found.value.filter((m) => m.key !== meta.key);
      emit("finished");
    }
  });
  if (!queue) error.value = "Another tab is uploading this recording already.";
}

async function discard(meta: RecordingMeta): Promise<void> {
  if (!window.confirm(`Throw away the ${LABEL[meta.version]} recording of “${meta.lectureTitle}”? This cannot be undone.`)) {
    return;
  }
  const result = await discardRecording(rec.chunks, meta);
  found.value = found.value.filter((m) => m.key !== meta.key);
  if (result === "pending") {
    error.value = "The server could not be reached. The recording is thrown away once it is back, the next time this page is opened.";
  } else if (result === "refused") {
    error.value = "The recording could not be thrown away: it is being converted already.";
  }
}

function describe(meta: RecordingMeta): string {
  const s = status[meta.key];
  if (!s) return "";
  if (s.state === "retrying") return "The server can't be reached; trying again…";
  if (s.state === "failed") return s.error;
  if (s.state === "done") return "Finished.";
  return `Uploading, ${(s.pendingBytes / 1024 ** 2).toFixed(0)} MB to go…`;
}
</script>

<template>
  <section
    v-if="found.length"
    class="flex flex-col gap-2 rounded-lg bg-warn/25 px-3 py-2 text-sm"
    aria-labelledby="recording-recovery-heading"
  >
    <h3 id="recording-recovery-heading" class="text-1 font-semibold">Unfinished recordings in this browser</h3>
    <p class="text-3">
      The page that recorded these was closed before everything was uploaded. Upload them now to finish the lectures.
    </p>
    <ul class="flex flex-col gap-1">
      <li v-for="meta in found" :key="meta.key" class="flex flex-wrap items-center gap-2">
        <span class="text-2 font-semibold">{{ meta.lectureTitle || "Untitled" }}</span>
        <span class="text-5">({{ LABEL[meta.version] }}, {{ new Date(meta.createdAt).toLocaleString() }})</span>
        <span class="text-5">{{ describe(meta) }}</span>
        <span class="ml-auto flex gap-1">
          <button
            v-if="!status[meta.key] || status[meta.key].state === 'failed'"
            type="button"
            class="tum-live-button-primary px-2 py-0.5 text-xs"
            @click="finish(meta)"
          >
            Upload and finish
          </button>
          <button
            type="button"
            class="tum-live-button-secondary tum-live-button px-2 py-0.5 text-xs"
            @click="saveLocalCopy(rec.chunks, meta)"
          >
            Save a copy
          </button>
          <button
            v-if="!status[meta.key]"
            type="button"
            class="tum-live-button-secondary tum-live-button px-2 py-0.5 text-xs"
            @click="discard(meta)"
          >
            Throw away
          </button>
        </span>
      </li>
    </ul>
    <p v-if="error" class="text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
  </section>
</template>
