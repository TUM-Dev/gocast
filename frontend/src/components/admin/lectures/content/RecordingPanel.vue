<script setup lang="ts">
import { computed, ref } from "vue";

import type { CourseLecture } from "@/lib/course-lectures";
import {
  SUBTITLE_LANGUAGES,
  downloadsOf,
  fileUrl,
  requestLectureSubtitles,
  subtitleErrorMessage,
} from "@/lib/lecture-content";

/**
 * The recording: its versions, the files to download, and asking for subtitles,
 * which the voice service generates from it and sends back when done.
 */
const props = defineProps<{
  courseId: number;
  lecture: CourseLecture;
}>();

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
    <p class="text-5 text-xs">
      Uploading or replacing a recording is still done on
      <a :href="`/admin/course/${courseId}#lecture-${lecture.id}`" class="underline hover:text-1">the course page</a>.
    </p>
  </section>
</template>
