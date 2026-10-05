<script setup lang="ts">
import { computed, ref } from "vue";

import {
  THUMBNAIL_ACCEPT,
  contentErrorMessage,
  customThumbnailOf,
  deleteLectureThumbnail,
  fileUrl,
  uploadLectureThumbnail,
  uploadProblem,
  type LectureFile,
} from "@/lib/lecture-content";

/**
 * The lecture's own thumbnail, shown in course listings instead of the frames taken
 * from the recording. Uploading one replaces the last.
 */
const props = defineProps<{
  courseId: number;
  lectureId: number;
  files: LectureFile[];
}>();

const emit = defineEmits<{ changed: [] }>();

// Every upload is a new file with a new ID, so the URL changes with it and no
// cache-busting is needed.
const current = computed(() => customThumbnailOf(props.files));

const busy = ref(false);
const error = ref("");
const status = ref("");
const input = ref<HTMLInputElement | null>(null);

async function upload(event: Event): Promise<void> {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file) return;
  error.value = "";
  status.value = "";
  const problem = uploadProblem(file, "thumbnail");
  if (problem) {
    error.value = problem;
    if (input.value) input.value.value = "";
    return;
  }
  busy.value = true;
  try {
    await uploadLectureThumbnail(props.courseId, props.lectureId, file);
    status.value = "Thumbnail uploaded.";
    emit("changed");
  } catch (err) {
    error.value = contentErrorMessage(err);
  } finally {
    busy.value = false;
    if (input.value) input.value.value = "";
  }
}

async function remove(): Promise<void> {
  if (!window.confirm("Delete the uploaded thumbnail? The one taken from the recording shows again.")) return;
  busy.value = true;
  error.value = "";
  status.value = "";
  try {
    await deleteLectureThumbnail(props.courseId, props.lectureId);
    status.value = "Thumbnail deleted.";
    emit("changed");
  } catch (err) {
    error.value = contentErrorMessage(err);
  } finally {
    busy.value = false;
  }
}

const inputId = computed(() => `lecture-${props.lectureId}-thumbnail-file`);
</script>

<template>
  <section class="flex flex-col gap-2 text-sm" :aria-labelledby="`lecture-${lectureId}-thumbnail-heading`">
    <h3
      :id="`lecture-${lectureId}-thumbnail-heading`"
      class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700"
    >
      Thumbnail
    </h3>

    <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-1" role="alert">{{ error }}</p>
    <p v-else-if="status" class="text-5" role="status">{{ status }}</p>

    <div class="flex flex-wrap items-start gap-3">
      <a v-if="current" :href="fileUrl(current.id, 'serve')" target="_blank" rel="noopener" class="block max-w-full">
        <img
          :src="fileUrl(current.id, 'serve')"
          alt="Custom thumbnail"
          class="aspect-video w-40 max-w-full rounded border object-cover dark:border-gray-700"
        />
      </a>
      <p v-else class="text-5">None uploaded; the thumbnails taken from the recording are shown.</p>
      <div class="flex flex-wrap items-center gap-2">
        <label
          :for="inputId"
          class="tum-live-button-secondary tum-live-button cursor-pointer px-3 py-1 text-sm focus-within:ring-2"
          :class="{ 'pointer-events-none opacity-50': busy }"
        >
          <i class="fas fa-image mr-1"></i>{{ current ? "Replace thumbnail" : "Upload thumbnail" }}
          <input
            :id="inputId"
            ref="input"
            type="file"
            :accept="THUMBNAIL_ACCEPT"
            class="sr-only"
            :disabled="busy"
            @change="upload"
          />
        </label>
        <button
          v-if="current"
          type="button"
          class="rounded border border-red-500 px-3 py-1 text-sm text-red-600 hover:bg-red-500 hover:text-white disabled:opacity-50 dark:text-red-400"
          :disabled="busy"
          @click="remove"
        >
          Delete thumbnail
        </button>
        <span v-if="busy" class="text-5" aria-live="polite"><i class="fas fa-circle-notch fa-spin mr-1"></i>Working…</span>
      </div>
    </div>
    <p class="text-5 text-xs">JPG, PNG, GIF or WebP, up to 50 MB.</p>
  </section>
</template>
