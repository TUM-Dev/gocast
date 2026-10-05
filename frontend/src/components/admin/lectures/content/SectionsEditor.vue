<script setup lang="ts">
import { computed, reactive, ref } from "vue";

import type { LectureSection } from "@/lib/course-lectures";
import {
  checkSection,
  contentErrorMessage,
  createLectureSections,
  deleteLectureSection,
  emptySectionForm,
  fileUrl,
  formatSectionStart,
  sectionChanged,
  sectionForm,
  updateLectureSection,
  type SectionForm,
} from "@/lib/lecture-content";

/**
 * A lecture's chapters: add one, change one, delete one. Each is saved at once, as
 * its own request, rather than collected into one save as the old card did.
 */
const props = defineProps<{
  courseId: number;
  lectureId: number;
  sections: LectureSection[];
}>();

const emit = defineEmits<{ changed: [] }>();

const busy = ref(false);
const error = ref("");
const status = ref("");

async function run(action: () => Promise<unknown>, done: string): Promise<boolean> {
  busy.value = true;
  error.value = "";
  status.value = "";
  try {
    await action();
    status.value = done;
    emit("changed");
    return true;
  } catch (err) {
    error.value = contentErrorMessage(err);
    return false;
  } finally {
    busy.value = false;
  }
}

/* Adding. */

const draft = reactive<SectionForm>(emptySectionForm());
/** Problems show once someone has tried to add, then follow the typing. */
const tried = ref(false);
const draftCheck = computed(() => checkSection(draft));
const draftProblems = computed(() => (tried.value && !draftCheck.value.ok ? draftCheck.value.problems : {}));

async function add(): Promise<void> {
  tried.value = true;
  const check = draftCheck.value;
  if (!check.ok) return;
  const ok = await run(
    () => createLectureSections(props.courseId, props.lectureId, [check.input]),
    `Section "${check.input.description}" added.`,
  );
  if (ok) {
    Object.assign(draft, emptySectionForm());
    tried.value = false;
  }
}

/* Editing, one section at a time. */

const editingId = ref<number | null>(null);
const editForm = reactive<SectionForm>(emptySectionForm());
const editCheck = computed(() => checkSection(editForm));
const editing = computed(() => props.sections.find((s) => s.id === editingId.value));
const editDirty = computed(
  () => !!editing.value && (!editCheck.value.ok || sectionChanged(editing.value, editCheck.value.input)),
);

function startEdit(s: LectureSection): void {
  editingId.value = s.id;
  Object.assign(editForm, sectionForm(s));
}

async function saveEdit(): Promise<void> {
  const check = editCheck.value;
  const id = editingId.value;
  if (!check.ok || id === null) return;
  const ok = await run(
    () => updateLectureSection(props.courseId, props.lectureId, id, check.input),
    `Section "${check.input.description}" saved.`,
  );
  if (ok) editingId.value = null;
}

function remove(s: LectureSection): void {
  if (!window.confirm(`Delete the section "${s.description}"?`)) return;
  if (editingId.value === s.id) editingId.value = null;
  void run(() => deleteLectureSection(props.courseId, props.lectureId, s.id), `Section "${s.description}" deleted.`);
}

const field = (name: string) => `lecture-${props.lectureId}-section-${name}`;
// Shrinkable, so the three fit a card as narrow as the phone layout makes it.
const timeInput = "tum-live-input w-16 min-w-0 shrink px-1 py-1 text-center";
</script>

<template>
  <section class="flex flex-col gap-2 text-sm" :aria-labelledby="field('heading')">
    <h3
      :id="field('heading')"
      class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700"
    >
      Video sections
    </h3>
    <p class="text-5 text-xs">Chapters make a recording easier to rewatch; viewers jump between them in the player.</p>

    <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-1" role="alert">{{ error }}</p>
    <p v-else-if="status" class="text-5" role="status">{{ status }}</p>

    <ul v-if="sections.length" class="flex flex-col gap-2" aria-label="Video sections">
      <li v-for="s in sections" :key="s.id" class="rounded border dark:border-gray-700">
        <form
          v-if="editingId === s.id"
          class="flex flex-col gap-2 p-2"
          :aria-label="`Edit section ${s.description}`"
          @submit.prevent="saveEdit"
        >
          <div class="flex flex-wrap items-end gap-2">
            <fieldset class="flex min-w-0 max-w-full items-center gap-1">
              <legend class="text-2 mb-1 text-xs">Start (h : m : s)</legend>
              <input v-model="editForm.hours" type="number" min="0" step="1" aria-label="Hours" :class="timeInput" />
              <span class="text-5">:</span>
              <input
                v-model="editForm.minutes"
                type="number"
                min="0"
                max="59"
                step="1"
                aria-label="Minutes"
                :class="timeInput"
              />
              <span class="text-5">:</span>
              <input
                v-model="editForm.seconds"
                type="number"
                min="0"
                max="59"
                step="1"
                aria-label="Seconds"
                :class="timeInput"
              />
            </fieldset>
            <div class="flex min-w-0 grow basis-40 flex-col">
              <label :for="field(`${s.id}-description`)" class="text-2 mb-1 text-xs">Description</label>
              <input
                :id="field(`${s.id}-description`)"
                v-model="editForm.description"
                type="text"
                autocomplete="off"
                class="tum-live-input w-full px-2 py-1"
              />
            </div>
          </div>
          <p v-if="!editCheck.ok" class="text-xs text-red-600 dark:text-red-400">
            {{ [editCheck.problems.time, editCheck.problems.description].filter(Boolean).join(" ") }}
          </p>
          <div class="flex justify-end gap-2">
            <button type="button" class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm" @click="editingId = null">
              Cancel
            </button>
            <button
              type="submit"
              class="tum-live-button-primary px-3 py-1 text-sm"
              :disabled="busy || !editCheck.ok || !editDirty"
            >
              Save section
            </button>
          </div>
        </form>
        <div v-else class="flex flex-wrap items-center gap-2 p-1.5">
          <img
            v-if="s.fileId"
            :src="fileUrl(s.fileId, 'serve')"
            alt=""
            loading="lazy"
            class="h-9 w-16 shrink-0 rounded object-cover"
          />
          <span
            class="shrink-0 rounded bg-sky-200 px-1.5 py-0.5 font-mono text-xs text-sky-800 dark:bg-indigo-800 dark:text-indigo-200"
          >
            {{ formatSectionStart(s) }}
          </span>
          <span class="text-3 min-w-32 grow basis-32 break-words font-semibold">{{ s.description }}</span>
          <span class="ml-auto flex shrink-0">
            <button
              type="button"
              class="text-4 hover:text-1 shrink-0 rounded px-2 py-1 hover:bg-gray-200 dark:hover:bg-gray-600"
              :aria-label="`Edit section ${s.description}`"
              :disabled="busy"
              @click="startEdit(s)"
            >
              <i class="fa fa-edit"></i>
            </button>
            <button
              type="button"
              class="text-4 shrink-0 rounded px-2 py-1 hover:bg-gray-200 hover:text-red-600 dark:hover:bg-gray-600"
              :aria-label="`Delete section ${s.description}`"
              :disabled="busy"
              @click="remove(s)"
            >
              <i class="fa fa-trash"></i>
            </button>
          </span>
        </div>
      </li>
    </ul>
    <p v-else class="text-5">No sections yet.</p>

    <form class="flex flex-col gap-2" aria-label="Add a section" novalidate @submit.prevent="add">
      <div class="flex flex-wrap items-end gap-2">
        <fieldset class="flex min-w-0 max-w-full items-center gap-1">
          <legend class="text-2 mb-1 text-xs">Start (h : m : s)</legend>
          <input
            v-model="draft.hours"
            type="number"
            min="0"
            step="1"
            placeholder="0"
            aria-label="Hours"
            :class="timeInput"
          />
          <span class="text-5">:</span>
          <input
            v-model="draft.minutes"
            type="number"
            min="0"
            max="59"
            step="1"
            placeholder="0"
            aria-label="Minutes"
            :class="timeInput"
          />
          <span class="text-5">:</span>
          <input
            v-model="draft.seconds"
            type="number"
            min="0"
            max="59"
            step="1"
            placeholder="0"
            aria-label="Seconds"
            :class="timeInput"
          />
        </fieldset>
        <div class="flex min-w-0 grow basis-40 flex-col">
          <label :for="field('new-description')" class="text-2 mb-1 text-xs">Description</label>
          <input
            :id="field('new-description')"
            v-model="draft.description"
            type="text"
            autocomplete="off"
            placeholder="Introduction"
            class="tum-live-input w-full px-2 py-1"
            :aria-invalid="!!draftProblems.description"
          />
        </div>
        <button type="submit" class="tum-live-button-primary px-3 py-1 text-sm" :disabled="busy">Add section</button>
      </div>
      <p v-if="draftProblems.time" class="text-xs text-red-600 dark:text-red-400">{{ draftProblems.time }}</p>
      <p v-if="draftProblems.description" class="text-xs text-red-600 dark:text-red-400">
        {{ draftProblems.description }}
      </p>
    </form>
  </section>
</template>
