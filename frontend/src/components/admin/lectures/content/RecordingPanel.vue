<script setup lang="ts">
import { computed, ref } from "vue";

import type { CourseLecture } from "@/lib/course-lectures";
import { isMp4, type MediaType } from "@/lib/create-lecture";
import {
  RECORDING_VERSIONS,
  SUBTITLE_LANGUAGES,
  downloadsOf,
  fileUrl,
  recordingUploadErrorMessage,
  requestLectureSubtitles,
  subtitleErrorMessage,
  uploadLectureRecording,
} from "@/lib/lecture-content";

/**
 * The recording: its versions, the files to download, uploading or replacing a
 * version, and asking for subtitles, which the voice service generates from it and
 * sends back when done.
 */
const props = defineProps<{
  courseId: number;
  lecture: CourseLecture;
}>();

const emit = defineEmits<{ changed: [] }>();

const downloads = computed(() => downloadsOf(props.lecture.files));

const requesting = ref(false);
const error = ref("");
const status = ref("");

async function requestSubtitles(code: string, label: string): Promise<void> {
  requesting.value = true;
  error.value = "";
  status.value = "";
  try {
    await requestLectureSubtitles(props.courseId, props.lecture.id, code);
    status.value = `${label} subtitles requested. They appear on the lecture once they have been generated.`;
  } catch (err) {
    error.value = subtitleErrorMessage(err);
  } finally {
    requesting.value = false;
  }
}

/* Uploading a version. A worker takes the file and transcodes it; the list shows the
 * progress once it reloads. Not offered while the lecture is live: the server would
 * refuse it, and the worker streaming it would throw the result away. */
const uploading = ref<MediaType | null>(null);
const uploadError = ref("");
const uploadStatus = ref("");

const hasVersion = (type: MediaType) => props.lecture.vodVersions.includes(type);

async function upload(type: MediaType, label: string, event: Event): Promise<void> {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  uploadError.value = "";
  uploadStatus.value = "";
  if (!isMp4(file)) {
    uploadError.value = "The recording must be an MP4 video.";
    input.value = "";
    return;
  }
  uploading.value = type;
  try {
    await uploadLectureRecording(props.courseId, props.lecture.id, type, file);
    uploadStatus.value = `${label} uploaded. It is being transcoded; the lecture shows the progress.`;
    emit("changed");
  } catch (err) {
    uploadError.value = recordingUploadErrorMessage(err);
  } finally {
    uploading.value = null;
    input.value = "";
  }
}
</script>

<template>
  <section class="flex flex-col gap-2 text-sm" :aria-labelledby="`lecture-${lecture.id}-recording-heading`">
    <h3
      :id="`lecture-${lecture.id}-recording-heading`"
      class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700"
    >
      Recording
    </h3>
    <dl class="text-3 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
      <dt class="text-5">Versions</dt>
      <dd>{{ lecture.vodVersions.length ? lecture.vodVersions.join(", ") : "None recorded" }}</dd>
      <template v-if="downloads.length">
        <dt class="text-5">Downloads</dt>
        <dd class="flex min-w-0 flex-wrap gap-x-3">
          <a
            v-for="file in downloads"
            :key="file.id"
            :href="fileUrl(file.id, 'download')"
            class="break-all underline hover:text-1"
            download
          >
            <i class="fas fa-cloud-download-alt mr-1"></i>{{ file.friendlyName }}
          </a>
        </dd>
      </template>
    </dl>

    <template v-if="lecture.recording">
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-2"><i class="fa-solid fa-closed-captioning mr-1"></i>Generate subtitles:</span>
        <button
          v-for="lang in SUBTITLE_LANGUAGES"
          :key="lang.code"
          type="button"
          class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm"
          :disabled="requesting"
          @click="requestSubtitles(lang.code, lang.label)"
        >
          {{ lang.label }}
        </button>
      </div>
      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-1" role="alert">{{ error }}</p>
      <p v-else-if="status" class="text-5" role="status">{{ status }}</p>
    </template>

    <p v-if="lecture.liveNow" class="text-5 text-xs">A recording can be uploaded once the lecture has ended.</p>
    <div v-else class="flex flex-col gap-1">
      <p class="text-2">{{ lecture.recording ? "Replace a version" : "Upload a recording" }} (MP4, H.264 if possible):</p>
      <div v-for="{ type, label } in RECORDING_VERSIONS" :key="type" class="flex min-w-0 flex-wrap items-center gap-2">
        <label :for="`lecture-${lecture.id}-recording-${type}`" class="text-3 w-40">
          {{ label }}<span v-if="hasVersion(type)" class="text-5"> (replace)</span>
        </label>
        <input
          :id="`lecture-${lecture.id}-recording-${type}`"
          type="file"
          accept="video/mp4,.mp4"
          class="text-3 min-w-0 max-w-full grow text-sm"
          :disabled="uploading !== null"
          @change="upload(type, label, $event)"
        />
        <span v-if="uploading === type" class="text-5 text-xs" aria-live="polite">
          <i class="fas fa-circle-notch fa-spin mr-1"></i>Uploading…
        </span>
      </div>
      <p v-if="uploadError" class="rounded-lg bg-danger/25 px-2 py-1" role="alert">{{ uploadError }}</p>
      <p v-else-if="uploadStatus" class="text-5" role="status">{{ uploadStatus }}</p>
    </div>
  </section>
</template>
