<script setup lang="ts">
import { ref, watch } from "vue";

import {
  fetchCourseVideoExport,
  videoExportScript,
  type CourseVideoExport,
  type VideoExportKind,
} from "@/lib/course-admin";
import { courseError } from "@/stores/course-admin";

/**
 * Download links for every recording of the course. The links open one at a time on
 * purpose: the edge prepares each file before sending it, so a script that fetches them
 * in turn is gentler on it than a browser opening all at once. The per-file links are for
 * single lectures; the edge names the file, as `download` is ignored cross-origin.
 */
const props = defineProps<{ courseId: number }>();

const kind = ref<VideoExportKind>("ALL");
const videos = ref<CourseVideoExport[] | null>(null);
const busy = ref(false);
const error = ref("");
const copied = ref(false);
// Bumped whenever the listing is invalidated, so a response that was in flight then is dropped.
let generation = 0;

// The tab stays mounted across courses; links of the previous one must not linger.
watch(
  () => props.courseId,
  () => {
    generation++;
    busy.value = false;
    videos.value = null;
    error.value = "";
    copied.value = false;
  },
);

// The list and the script are of the version that was fetched, not the one now selected.
watch(kind, () => {
  videos.value = null;
  copied.value = false;
});

async function load(): Promise<void> {
  const mine = ++generation;
  busy.value = true;
  error.value = "";
  copied.value = false;
  try {
    const result = await fetchCourseVideoExport(props.courseId, kind.value);
    if (mine === generation) videos.value = result;
  } catch (err) {
    if (mine !== generation) return;
    videos.value = null;
    error.value = courseError(err);
  } finally {
    if (mine === generation) busy.value = false;
  }
}

async function copyScript(): Promise<void> {
  if (!videos.value) return;
  try {
    await navigator.clipboard.writeText(videoExportScript(videos.value));
    copied.value = true;
  } catch {
    error.value = "Could not copy to the clipboard.";
  }
}
</script>

<template>
  <div class="flex flex-col gap-3 text-sm">
    <p class="text-3">
      Lists a download link for every recorded lecture, in every available version unless you pick one. The links expire after a few hours; list them again if they
      stop working.
    </p>

    <p
      v-if="error"
      class="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
      role="alert"
    >
      {{ error }}
    </p>

    <div class="flex flex-wrap items-end gap-2">
      <div class="flex flex-col gap-1">
        <label for="cs-export-kind" class="text-2">Version</label>
        <select id="cs-export-kind" v-model="kind" class="tum-live-input" :disabled="busy">
          <option value="ALL">All versions</option>
          <option value="COMBINED">Combined</option>
          <option value="PRESENTATION">Presentation</option>
          <option value="CAMERA">Camera</option>
        </select>
      </div>
      <button type="button" class="tum-live-button-secondary tum-live-button px-4 py-2 text-sm" :disabled="busy" @click="load">
        <i class="fa-solid fa-download mr-2" aria-hidden="true"></i>{{ busy ? "Listing…" : "List video downloads" }}
      </button>
    </div>

    <template v-if="videos">
      <p v-if="videos.length === 0" class="text-3" role="status">This course has no downloadable recordings in this version.</p>
      <template v-else>
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-2">{{ videos.length }} {{ videos.length === 1 ? "video" : "videos" }}</span>
          <button type="button" class="tum-live-button-secondary tum-live-button px-3 py-1 text-xs" @click="copyScript">
            <i class="fa-regular fa-copy mr-2" aria-hidden="true"></i>Copy download script
          </button>
          <span v-if="copied" class="text-xs text-green-700 dark:text-green-300" role="status">Copied.</span>
        </div>
        <ul class="flex max-h-72 flex-col divide-y overflow-y-auto rounded-lg border dark:divide-gray-800 dark:border-gray-800">
          <li v-for="v in videos" :key="v.fileName" class="flex items-center justify-between gap-3 px-3 py-2">
            <span class="text-1 truncate">{{ v.fileName }}</span>
            <a :href="v.url" class="shrink-0 text-blue-600 hover:underline dark:text-blue-400" rel="noopener">Download</a>
          </li>
        </ul>
      </template>
    </template>
  </div>
</template>
