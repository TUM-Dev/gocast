<script setup lang="ts">
import { computed, ref } from "vue";

import {
  attachmentsOf,
  contentErrorMessage,
  deleteLectureAttachment,
  fileUrl,
  uploadLectureAttachment,
  uploadProblem,
  type LectureFile,
} from "@/lib/lecture-content";

/**
 * The files students can download from the lecture's page. Uploaded as soon as they
 * are picked, one after the other.
 *
 * The list carries no file sizes, so only names are shown.
 */
const props = defineProps<{
  courseId: number;
  lectureId: number;
  files: LectureFile[];
}>();

const emit = defineEmits<{ changed: [] }>();

const attachments = computed(() => attachmentsOf(props.files));

/** The name of the file being uploaded, "" when idle. */
const uploading = ref("");
const deleting = ref(false);
const error = ref("");
const status = ref("");
const input = ref<HTMLInputElement | null>(null);

async function upload(event: Event): Promise<void> {
  const picked = Array.from((event.target as HTMLInputElement).files ?? []);
  error.value = "";
  status.value = "";
  const done: string[] = [];
  try {
    for (const file of picked) {
      const problem = uploadProblem(file, "attachment");
      if (problem) {
        error.value = `${file.name}: ${problem}`;
        break;
      }
      uploading.value = file.name;
      await uploadLectureAttachment(props.courseId, props.lectureId, file);
      done.push(file.name);
    }
  } catch (err) {
    error.value = `${uploading.value}: ${contentErrorMessage(err)}`;
  } finally {
    uploading.value = "";
    if (input.value) input.value.value = "";
  }
  if (done.length) {
    status.value = done.length === 1 ? `Uploaded ${done[0]}.` : `Uploaded ${done.length} files.`;
    emit("changed");
  }
}

async function remove(file: LectureFile): Promise<void> {
  if (!window.confirm(`Delete the attachment "${file.friendlyName}"?`)) return;
  deleting.value = true;
  error.value = "";
  status.value = "";
  try {
    await deleteLectureAttachment(props.courseId, props.lectureId, file.id);
    status.value = `Deleted ${file.friendlyName}.`;
    emit("changed");
  } catch (err) {
    error.value = contentErrorMessage(err);
  } finally {
    deleting.value = false;
  }
}

const inputId = computed(() => `lecture-${props.lectureId}-attachment-file`);
</script>

<template>
  <section class="flex flex-col gap-2 text-sm" :aria-labelledby="`lecture-${lectureId}-attachments-heading`">
    <h3
      :id="`lecture-${lectureId}-attachments-heading`"
      class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700"
    >
      Attachments
    </h3>

    <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-1" role="alert">{{ error }}</p>
    <p v-else-if="status" class="text-5" role="status">{{ status }}</p>

    <ul v-if="attachments.length" class="flex flex-col gap-1" aria-label="Attachments">
      <li v-for="file in attachments" :key="file.id" class="flex items-center gap-2">
        <i class="fas fa-paperclip text-5"></i>
        <a :href="fileUrl(file.id, 'download')" class="text-3 min-w-0 grow break-all hover:underline" download>
          {{ file.friendlyName }}
        </a>
        <button
          type="button"
          class="text-4 shrink-0 rounded px-2 py-1 hover:bg-gray-200 hover:text-red-600 dark:hover:bg-gray-600"
          :aria-label="`Delete attachment ${file.friendlyName}`"
          :disabled="deleting || !!uploading"
          @click="remove(file)"
        >
          <i class="fa fa-xmark"></i>
        </button>
      </li>
    </ul>
    <p v-else class="text-5">No attachments.</p>

    <div class="flex flex-wrap items-center gap-3">
      <!-- The label is the visible button; the input stays reachable by keyboard. -->
      <label
        :for="inputId"
        class="tum-live-button-secondary tum-live-button cursor-pointer px-3 py-1 text-sm focus-within:ring-2"
        :class="{ 'pointer-events-none opacity-50': uploading }"
      >
        <i class="fas fa-upload mr-1"></i>Upload attachments
        <input
          :id="inputId"
          ref="input"
          type="file"
          multiple
          class="sr-only"
          :disabled="!!uploading"
          @change="upload"
        />
      </label>
      <span v-if="uploading" class="text-5 flex min-w-0 items-center gap-2" aria-live="polite">
        <i class="fas fa-circle-notch fa-spin"></i>
        <span class="break-all">Uploading {{ uploading }}…</span>
      </span>
      <span v-else class="text-5 text-xs">Up to 50 MB each.</span>
    </div>
  </section>
</template>
