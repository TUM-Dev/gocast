<script setup lang="ts">
import { onMounted, ref } from "vue";

import type { BrowserRecording } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import { abandonBrowserRecording, finalizeBrowserRecording, listBrowserRecordings } from "@/lib/recorder/api";
import { CHUNK_MS } from "@/lib/recorder/session";

/**
 * The lecture's browser recordings that no runner has yet, as the server sees them:
 * one still being recorded or waiting for its browser, or one whose hand-off to a
 * runner failed. One whose browser is gone for good can be finished with what arrived.
 */
const props = defineProps<{ courseId: number; lectureId: number }>();
const emit = defineEmits<{ changed: [] }>();

const recordings = ref<BrowserRecording[]>([]);
const error = ref("");
const working = ref(false);

const LABEL: Record<string, string> = { PRES: "Screen", CAM: "Camera" };

async function load(): Promise<void> {
  try {
    recordings.value = await listBrowserRecordings(props.courseId, props.lectureId);
  } catch {
    recordings.value = [];
  }
}

onMounted(load);

function arrived(r: BrowserRecording): string {
  const seconds = (r.received.reduce((n, c) => n + c, 0) * CHUNK_MS) / 1000;
  const m = Math.floor(seconds / 60);
  return m > 0 ? `about ${m} min` : `${Math.round(seconds)} s`;
}

function describe(r: BrowserRecording): string {
  if (!r.finalized) return `Recording or waiting for its browser; ${arrived(r)} arrived.`;
  if (r.handOffGaveUp) return `Could not be handed to a runner: ${r.handOffError}`;
  if (r.handedOff) return "Being converted.";
  if (r.handOffError) return `Being handed to a runner, which failed last time: ${r.handOffError}`;
  return "Being handed to a runner.";
}

async function act(run: () => Promise<unknown>): Promise<void> {
  working.value = true;
  error.value = "";
  try {
    await run();
    emit("changed");
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : "Something went wrong. Please try again.";
  } finally {
    working.value = false;
    await load();
  }
}

function finish(r: BrowserRecording): Promise<void> {
  const ask =
    "Finish this recording with what arrived? Only do this if the browser that recorded it is gone: " +
    "anything it still holds can't be added afterwards.";
  if (!r.finalized && !window.confirm(ask)) return Promise.resolve();
  return act(() => finalizeBrowserRecording(props.courseId, props.lectureId, r.id));
}

function discard(r: BrowserRecording): Promise<void> {
  if (!window.confirm(`Throw away the ${LABEL[r.version]?.toLowerCase()} recording? This cannot be undone.`)) {
    return Promise.resolve();
  }
  return act(() => abandonBrowserRecording(props.courseId, props.lectureId, r.id));
}
</script>

<template>
  <div v-if="recordings.length" class="flex flex-col gap-1">
    <h4 class="text-2 font-semibold">Recorded in the browser</h4>
    <ul class="flex flex-col gap-1">
      <li v-for="r in recordings" :key="r.id" class="flex flex-wrap items-center gap-2">
        <span class="text-2 w-16">{{ LABEL[r.version] ?? r.version }}</span>
        <span class="text-5">{{ describe(r) }}</span>
        <span class="ml-auto flex gap-1">
          <button
            v-if="!r.finalized || r.handOffGaveUp"
            type="button"
            class="tum-live-button-secondary tum-live-button px-2 py-0.5 text-xs"
            :disabled="working"
            @click="finish(r)"
          >
            {{ r.finalized ? "Try again" : "Finish with what arrived" }}
          </button>
          <button
            v-if="!r.finalized || r.handOffGaveUp"
            type="button"
            class="tum-live-button-secondary tum-live-button px-2 py-0.5 text-xs"
            :disabled="working"
            @click="discard(r)"
          >
            Throw away
          </button>
        </span>
      </li>
    </ul>
    <p v-if="error" class="text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
  </div>
</template>
