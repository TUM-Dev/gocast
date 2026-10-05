<script setup lang="ts">
import DOMPurify from "dompurify";
import { marked } from "marked";
import { computed, reactive, ref, watch } from "vue";

import LectureCopyForm from "@/components/admin/lectures/LectureCopyForm.vue";
import StreamKeyInfo from "@/components/admin/lectures/StreamKeyInfo.vue";
import {
  FILE_TYPE_ATTACHMENT,
  deleteLectureSeries,
  deleteLectures,
  lectureErrorMessage,
  updateLecture,
  updateLectureSeries,
  updateLectureSeriesTime,
  type AdministeredCourse,
  type CourseLecture,
  type SeriesChanges,
} from "@/lib/course-lectures";
import { fromDateTimeLocal, toDateTimeLocal } from "@/lib/datetime-local";
import { SELF_STREAMED, type ScheduleLectureHall } from "@/lib/schedule";

/**
 * The expanded half of a lecture's card: everything about the lecture that can be
 * changed, one form per group, each saved on its own.
 *
 * After every change the parent reloads the list rather than patching the lecture
 * here, because a series change touches lectures this editor does not hold.
 */
const props = defineProps<{
  lecture: CourseLecture;
  /** The lectures in its series, itself included; 0 when it is in none. */
  seriesCount: number;
  halls: ScheduleLectureHall[];
  courseId: number;
  /** For the self-streaming key; empty until the course has loaded. */
  courseSlug: string;
  /** The other courses the caller administers, for copy and move. */
  targetCourses: AdministeredCourse[];
}>();

const emit = defineEmits<{
  /** Something about this lecture or its series changed; reload. */
  changed: [];
}>();

const id = computed(() => props.lecture.id);
const inSeries = computed(() => props.seriesCount > 0);

const edit = reactive({
  name: props.lecture.name,
  description: props.lecture.description,
  start: toDateTimeLocal(props.lecture.start),
  end: toDateTimeLocal(props.lecture.end),
  hallId: props.lecture.lectureHallId,
  chatEnabled: props.lecture.chatEnabled,
  private: props.lecture.private,
});
const series = reactive({ text: false, time: false, hall: false, chat: false });

/*
 * A reload hands in a new lecture. Fields still showing the old value follow it; a
 * field someone has typed into keeps what they typed, so saving one group does not
 * throw away an unsaved edit in another.
 */
watch(
  () => props.lecture,
  (now, before) => {
    const follow = <K extends keyof typeof edit>(key: K, was: (typeof edit)[K], is: (typeof edit)[K]) => {
      if (edit[key] === was) edit[key] = is;
    };
    follow("name", before.name, now.name);
    follow("description", before.description, now.description);
    follow("start", toDateTimeLocal(before.start), toDateTimeLocal(now.start));
    follow("end", toDateTimeLocal(before.end), toDateTimeLocal(now.end));
    follow("hallId", before.lectureHallId, now.lectureHallId);
    follow("chatEnabled", before.chatEnabled, now.chatEnabled);
    follow("private", before.private, now.private);
  },
);

const saving = ref(false);
const saved = ref("");
const saveError = ref("");

async function run(action: () => Promise<unknown>, done: string): Promise<boolean> {
  saving.value = true;
  saved.value = "";
  saveError.value = "";
  try {
    await action();
    saved.value = done;
    emit("changed");
    return true;
  } catch (err) {
    saveError.value = lectureErrorMessage(err);
    return false;
  } finally {
    saving.value = false;
  }
}

/* Title and description. */

const textDirty = computed(
  () => edit.name.trim() !== props.lecture.name || edit.description !== props.lecture.description,
);

function saveText(): void {
  const changes: SeriesChanges = {};
  if (edit.name.trim() !== props.lecture.name) changes.name = edit.name;
  if (edit.description !== props.lecture.description) changes.description = edit.description;
  const toSeries = inSeries.value && series.text;
  void run(
    () =>
      toSeries
        ? updateLectureSeries(props.courseId, id.value, changes)
        : updateLecture(props.courseId, id.value, changes),
    toSeries ? `Saved for all ${props.seriesCount} lectures of the series.` : "Title and description saved.",
  );
}

const showPreview = ref(false);
// Client-side only, so it approximates the server's rendering of the description;
// sanitized because the description is anyone-with-admin-rights input.
const preview = computed(() =>
  edit.description.trim()
    ? DOMPurify.sanitize(marked.parse(edit.description, { async: false }) as string)
    : "",
);

/* Schedule. */

const startDate = computed(() => fromDateTimeLocal(edit.start));
const endDate = computed(() => fromDateTimeLocal(edit.end));
const timeDirty = computed(
  () =>
    edit.start !== toDateTimeLocal(props.lecture.start) ||
    edit.end !== toDateTimeLocal(props.lecture.end),
);
const timeProblem = computed(() => {
  if (!startDate.value || !endDate.value) return "Enter both a start and an end.";
  if (endDate.value <= startDate.value) return "The end must be after the start.";
  return "";
});

function saveTime(): void {
  const start = startDate.value;
  const end = endDate.value;
  if (!start || !end || timeProblem.value) return;
  const toSeries = inSeries.value && series.time;
  void run(
    () =>
      toSeries
        ? updateLectureSeriesTime(props.courseId, id.value, start, end)
        : updateLecture(props.courseId, id.value, { start, end }),
    toSeries
      ? `Time saved; the other ${props.seriesCount - 1} lectures of the series moved to the same time of day.`
      : "Time saved.",
  );
}

/* Lecture hall. */

const hallDirty = computed(() => edit.hallId !== props.lecture.lectureHallId);

function saveHall(): void {
  const toSeries = inSeries.value && series.hall;
  const changes = { lectureHallId: edit.hallId };
  void run(
    () =>
      toSeries
        ? updateLectureSeries(props.courseId, id.value, changes)
        : updateLecture(props.courseId, id.value, changes),
    toSeries ? `Lecture hall saved for all ${props.seriesCount} lectures of the series.` : "Lecture hall saved.",
  );
}

/* Chat and visibility. */

const optionsDirty = computed(
  () => edit.chatEnabled !== props.lecture.chatEnabled || edit.private !== props.lecture.private,
);

async function saveOptions(): Promise<void> {
  const chatChanged = edit.chatEnabled !== props.lecture.chatEnabled;
  const privateChanged = edit.private !== props.lecture.private;
  // Visibility has no series RPC, so a series chat change and a visibility change are
  // two requests; the first failure stops the second.
  await run(async () => {
    if (chatChanged && inSeries.value && series.chat) {
      await updateLectureSeries(props.courseId, id.value, { chatEnabled: edit.chatEnabled });
      if (privateChanged) await updateLecture(props.courseId, id.value, { private: edit.private });
      return;
    }
    await updateLecture(props.courseId, id.value, {
      chatEnabled: chatChanged ? edit.chatEnabled : undefined,
      private: privateChanged ? edit.private : undefined,
    });
  }, "Options saved.");
}

/* Deleting. */

function deleteLecture(): void {
  if (!window.confirm(`Delete "${props.lecture.name || "this lecture"}"? This includes any recordings.`)) return;
  void run(() => deleteLectures(props.courseId, [id.value]), "Lecture deleted.");
}

function deleteSeries(): void {
  if (
    !window.confirm(
      `Delete all ${props.seriesCount} lectures of this series? This includes any recordings.`,
    )
  )
    return;
  void run(() => deleteLectureSeries(props.courseId, id.value), "Series deleted.");
}

/* What the list carries about content, shown read-only. */

const attachments = computed(() => props.lecture.files.filter((f) => f.type === FILE_TYPE_ATTACHMENT));

const field = (name: string) => `lecture-${id.value}-${name}`;
</script>

<template>
  <div class="flex flex-col gap-5 border-t p-4 dark:border-gray-700">
    <p v-if="saveError" class="rounded-lg bg-danger/25 px-2 py-1 text-sm" role="alert">
      {{ saveError }}
    </p>
    <p v-else-if="saved" class="text-5 text-sm" role="status">{{ saved }}</p>

    <!-- Title and description -->
    <form class="flex flex-col gap-2 text-sm" @submit.prevent="saveText">
      <h3 class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700">
        Title and description
      </h3>
      <label :for="field('name')" class="text-2">Lecture title</label>
      <input
        :id="field('name')"
        v-model="edit.name"
        type="text"
        autocomplete="off"
        placeholder="Lecture 2: Dark-Patterns I"
        class="tum-live-input"
      />
      <div class="flex items-center justify-between gap-2">
        <label :for="field('description')" class="text-2">Description</label>
        <button type="button" class="text-5 hover:text-1 text-xs" @click="showPreview = !showPreview">
          {{ showPreview ? "Edit" : "Preview" }}
        </button>
      </div>
      <textarea
        v-show="!showPreview"
        :id="field('description')"
        v-model="edit.description"
        rows="4"
        placeholder="Add a nice description, links, and more. You can use Markdown."
        class="tum-live-input"
      ></textarea>
      <template v-if="showPreview">
        <!-- Sanitized by DOMPurify above. -->
        <div
          v-if="preview"
          class="lecture-description-preview text-3 min-h-16 rounded border p-2 dark:border-gray-700"
          v-html="preview"
        ></div>
        <p v-else class="text-5 rounded border p-2 dark:border-gray-700">Nothing to preview.</p>
      </template>
      <div class="flex flex-wrap items-center justify-end gap-3">
        <label v-if="inSeries" class="text-3 mr-auto flex items-center gap-2">
          <input v-model="series.text" type="checkbox" />
          Apply to all {{ seriesCount }} lectures of the series
        </label>
        <button type="submit" class="tum-live-button-primary px-3 py-1 text-sm" :disabled="saving || !textDirty">
          Save
        </button>
      </div>
    </form>

    <!-- Schedule -->
    <form class="flex flex-col gap-2 text-sm" @submit.prevent="saveTime">
      <h3 class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700">
        Schedule
      </h3>
      <div class="grid gap-3 sm:grid-cols-2">
        <div class="flex min-w-0 flex-col gap-1">
          <label :for="field('start')" class="text-2">Start</label>
          <input :id="field('start')" v-model="edit.start" type="datetime-local" class="tum-live-input w-full" />
        </div>
        <div class="flex min-w-0 flex-col gap-1">
          <label :for="field('end')" class="text-2">End</label>
          <input :id="field('end')" v-model="edit.end" type="datetime-local" class="tum-live-input w-full" />
        </div>
      </div>
      <p v-if="timeDirty && timeProblem" class="text-sm text-red-600 dark:text-red-400">{{ timeProblem }}</p>
      <div class="flex flex-wrap items-center justify-end gap-3">
        <label v-if="inSeries" class="text-3 mr-auto flex items-center gap-2">
          <input v-model="series.time" type="checkbox" />
          Move the other {{ seriesCount - 1 }} lectures of the series to the same time of day
        </label>
        <button
          type="submit"
          class="tum-live-button-primary px-3 py-1 text-sm"
          :disabled="saving || !timeDirty || !!timeProblem"
        >
          Save
        </button>
      </div>
    </form>

    <!-- Lecture hall -->
    <form class="flex flex-col gap-2 text-sm" @submit.prevent="saveHall">
      <h3 class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700">
        Lecture hall
      </h3>
      <label :for="field('hall')" class="text-2">Streamed from</label>
      <select :id="field('hall')" v-model.number="edit.hallId" class="tum-live-input">
        <option :value="SELF_STREAMED">None (self-streamed)</option>
        <option v-for="hall in halls" :key="hall.id" :value="hall.id">{{ hall.name }}</option>
        <!-- A hall the list names but the hall listing does not, so the select never shows blank. -->
        <option
          v-if="lecture.lectureHallId && !halls.some((h) => h.id === lecture.lectureHallId)"
          :value="lecture.lectureHallId"
        >
          {{ lecture.lectureHallName || `Hall ${lecture.lectureHallId}` }}
        </option>
      </select>
      <StreamKeyInfo
        v-if="edit.hallId === SELF_STREAMED && !lecture.recording"
        :lecture-id="lecture.id"
        :course-slug="courseSlug"
        :stream-key="lecture.streamKey"
        :pending="hallDirty"
      />
      <div class="flex flex-wrap items-center justify-end gap-3">
        <label v-if="inSeries" class="text-3 mr-auto flex items-center gap-2">
          <input v-model="series.hall" type="checkbox" />
          Apply to all {{ seriesCount }} lectures of the series
        </label>
        <button type="submit" class="tum-live-button-primary px-3 py-1 text-sm" :disabled="saving || !hallDirty">
          Save
        </button>
      </div>
    </form>

    <!-- Chat and visibility -->
    <form class="flex flex-col gap-2 text-sm" @submit.prevent="saveOptions">
      <h3 class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700">
        Options
      </h3>
      <label class="text-3 flex items-center gap-2">
        <input v-model="edit.chatEnabled" type="checkbox" />
        Chat enabled
      </label>
      <label v-if="inSeries" class="text-3 ml-6 flex items-center gap-2">
        <input v-model="series.chat" type="checkbox" />
        Apply the chat setting to all {{ seriesCount }} lectures of the series
      </label>
      <label class="text-3 flex items-center gap-2">
        <input v-model="edit.private" type="checkbox" />
        Private: only the course's administrators can see this lecture
      </label>
      <div class="flex justify-end">
        <button type="submit" class="tum-live-button-primary px-3 py-1 text-sm" :disabled="saving || !optionsDirty">
          Save
        </button>
      </div>
    </form>

    <!-- Recording and content, read-only here. -->
    <section class="flex flex-col gap-2 text-sm">
      <h3 class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700">
        Recording and content
      </h3>
      <!--
        Extension point: uploading or replacing videos, attachments, a custom
        thumbnail, editing sections, requesting subtitles and VoD downloads land here
        with their RPCs (lecture content, part 2). Until then they are on the
        server-rendered course page.
      -->
      <dl class="text-3 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
        <dt class="text-5">Versions</dt>
        <dd>{{ lecture.vodVersions.length ? lecture.vodVersions.join(", ") : "None recorded" }}</dd>
        <dt class="text-5">Attachments</dt>
        <dd>
          <template v-if="attachments.length">
            {{ attachments.map((f) => f.friendlyName).join(", ") }}
          </template>
          <template v-else>None</template>
        </dd>
        <dt class="text-5">Sections</dt>
        <dd>{{ lecture.videoSectionCount }}</dd>
      </dl>
      <p class="text-5 text-xs">
        Videos, attachments, the thumbnail, sections and subtitles are still managed on
        <a :href="`/admin/course/${courseId}#lecture-${lecture.id}`" class="underline hover:text-1">the
          course page</a>.
      </p>
    </section>

    <!-- Copy or move -->
    <LectureCopyForm
      :course-id="courseId"
      :lecture-id="lecture.id"
      :target-courses="targetCourses"
      @changed="emit('changed')"
    />

    <!-- Deleting -->
    <section class="flex flex-col gap-2 text-sm">
      <h3 class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700">
        Delete
      </h3>
      <div class="flex flex-wrap gap-2">
        <button
          type="button"
          class="rounded border border-red-500 px-3 py-1 text-sm text-red-600 hover:bg-red-500 hover:text-white disabled:opacity-50 dark:text-red-400"
          :disabled="saving"
          @click="deleteLecture"
        >
          Delete lecture
        </button>
        <button
          v-if="inSeries"
          type="button"
          class="rounded border border-red-500 px-3 py-1 text-sm text-red-600 hover:bg-red-500 hover:text-white disabled:opacity-50 dark:text-red-400"
          :disabled="saving"
          @click="deleteSeries"
        >
          Delete series ({{ seriesCount }} lectures)
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.lecture-description-preview :deep(a) {
  text-decoration: underline;
}

.lecture-description-preview :deep(ul) {
  list-style: disc;
  padding-left: 1.5rem;
}

.lecture-description-preview :deep(ol) {
  list-style: decimal;
  padding-left: 1.5rem;
}
</style>
