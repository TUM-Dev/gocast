<script setup lang="ts">
import { computed, ref } from "vue";

import { copyLecture, lectureErrorMessage, type AdministeredCourse } from "@/lib/course-lectures";

/** Copies or moves a lecture into another course the caller administers. */
const props = defineProps<{
  courseId: number;
  lectureId: number;
  /** Excludes the lecture's own course. */
  targetCourses: AdministeredCourse[];
}>();

const emit = defineEmits<{
  /** A move removed the lecture from this course. */
  changed: [];
}>();

const target = ref(0);
const move = ref(false);
const busy = ref(false);
const done = ref("");
const error = ref("");

const targetName = computed(() => props.targetCourses.find((c) => c.id === target.value)?.name ?? "");
const field = (name: string) => `lecture-${props.lectureId}-${name}`;

async function submit(): Promise<void> {
  if (!target.value) return;
  if (move.value && !window.confirm(`Move this lecture to ${targetName.value}? It leaves this course.`)) return;
  busy.value = true;
  done.value = "";
  error.value = "";
  try {
    await copyLecture(props.courseId, props.lectureId, target.value, move.value);
    done.value = move.value ? `Moved to ${targetName.value}.` : `Copied to ${targetName.value}.`;
    if (move.value) emit("changed");
  } catch (err) {
    error.value = lectureErrorMessage(err);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <form class="flex flex-col gap-2 text-sm" @submit.prevent="submit">
    <h3 class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700">
      Copy or move to another course
    </h3>
    <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-1 text-sm" role="alert">{{ error }}</p>
    <p v-else-if="done" class="text-5 text-sm" role="status">{{ done }}</p>
    <p v-if="!targetCourses.length" class="text-5">You administer no other course.</p>
    <template v-else>
      <label :for="field('target')" class="text-2">Target course</label>
      <select :id="field('target')" v-model.number="target" class="tum-live-input">
        <option :value="0" disabled>Choose a course</option>
        <option v-for="c in targetCourses" :key="c.id" :value="c.id">
          {{ c.name }} ({{ c.term === "W" ? "Winter" : "Summer" }} {{ c.year }})
        </option>
      </select>
      <div class="flex flex-wrap items-center justify-end gap-3">
        <fieldset class="text-3 mr-auto flex gap-4">
          <legend class="sr-only">Copy or move</legend>
          <label class="flex items-center gap-1"><input v-model="move" type="radio" :value="false" /> Copy</label>
          <label class="flex items-center gap-1"><input v-model="move" type="radio" :value="true" /> Move</label>
        </fieldset>
        <button type="submit" class="tum-live-button-primary px-3 py-1 text-sm" :disabled="busy || !target">
          {{ move ? "Move" : "Copy" }}
        </button>
      </div>
    </template>
  </form>
</template>
