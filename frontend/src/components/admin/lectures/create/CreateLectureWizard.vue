<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";

import CreateLectureDetailsStep from "@/components/admin/lectures/create/CreateLectureDetailsStep.vue";
import CreateLectureFilesStep from "@/components/admin/lectures/create/CreateLectureFilesStep.vue";
import CreateLectureTypeStep from "@/components/admin/lectures/create/CreateLectureTypeStep.vue";
import { updateLecture, type CourseLecture } from "@/lib/course-lectures";
import {
  MEDIA_TYPES,
  buildCreatePlan,
  createErrorMessage,
  createLectures,
  detailsProblems,
  emptyForm,
  filesProblems,
  lectureCount,
  mediaUploadErrorMessage,
  seriesRows,
  uploadLectureMedia,
} from "@/lib/create-lecture";
import { fromDateTimeLocal, toDateTimeLocal } from "@/lib/datetime-local";
import type { ScheduleLectureHall } from "@/lib/schedule";

/**
 * The old edit-course page's "create lecture" wizard: the type, then the details,
 * then for a video upload the files. Creates the lectures with one createLectures
 * call, gives the lectures of a series that have one their own title, and uploads a
 * video upload's files to the new lecture one after another.
 *
 * Unlike v1, a failed upload does not delete the lecture again: it is there to be
 * uploaded to later, and the message says so.
 */
const props = defineProps<{
  courseId: number;
  halls: ScheduleLectureHall[];
  canChooseHall: boolean;
  courseChatEnabled: boolean;
}>();

const emit = defineEmits<{
  /** The lectures now in the course; `problem` when something after creating failed. */
  created: [lectures: CourseLecture[], problem: string];
  close: [];
}>();

const form = reactive(emptyForm(props.courseChatEnabled));

type Step = "type" | "details" | "files";
const STEP_TITLES: Record<Step, string> = { type: "Type", details: "Details", files: "Video files" };
const steps = computed<Step[]>(() => (form.mode === "vod" ? ["type", "details", "files"] : ["type", "details"]));
const stepIndex = ref(0);
const step = computed(() => steps.value[stepIndex.value]);
const onLastStep = computed(() => stepIndex.value === steps.value.length - 1);

/** Problems are shown once someone tried to go on, not while the form is still empty. */
const attempted = ref(false);
const problems = computed(() => {
  if (step.value === "details") return detailsProblems(form, new Date());
  if (step.value === "files") return filesProblems(form);
  return [];
});

/* The series list follows the start, the interval and the count. */
watch(
  () => [form.recurring, form.start, form.interval, form.count] as const,
  () => {
    const first = fromDateTimeLocal(form.start);
    form.rows = form.recurring && first ? seriesRows(first, form.interval, form.count, form.rows) : [];
  },
);

/* A sensible end once a start is set, as most lectures last 90 minutes. */
watch(
  () => form.start,
  (start) => {
    const begin = fromDateTimeLocal(start);
    if (begin && !form.end) form.end = toDateTimeLocal(new Date(begin.getTime() + 90 * 60_000));
  },
);

const busy = ref("");
const error = ref("");

const submitLabel = computed(() => {
  if (!onLastStep.value) return "Continue";
  const n = lectureCount(form);
  return n > 1 ? `Create ${n} lectures` : "Create lecture";
});

function back(): void {
  attempted.value = false;
  error.value = "";
  stepIndex.value = Math.max(0, stepIndex.value - 1);
}

async function next(): Promise<void> {
  attempted.value = true;
  if (problems.value.length) return;
  if (!onLastStep.value) {
    attempted.value = false;
    stepIndex.value++;
    return;
  }
  await submit();
}

async function submit(): Promise<void> {
  error.value = "";
  const plan = buildCreatePlan(props.courseId, form, new Date());
  const n = lectureCount(form);
  busy.value = n > 1 ? `Creating ${n} lectures…` : "Creating the lecture…";

  let created: CourseLecture[];
  try {
    created = await createLectures(props.courseId, plan.request);
  } catch (err) {
    // Nothing was created (the server creates all or none): stay, so it can be fixed.
    error.value = createErrorMessage(err);
    busy.value = "";
    return;
  }

  let problem = "";
  const renames = created.map((l, i) => [l, plan.titles[i] ?? ""] as const).filter(([, title]) => title);
  for (const [lecture, title] of renames) {
    try {
      await updateLecture(props.courseId, lecture.id, { name: title });
      lecture.name = title;
    } catch {
      problem = "The lectures were created, but not every one got its own title. Rename them from their cards.";
    }
  }

  if (form.mode === "vod" && created[0]) {
    const uploads = MEDIA_TYPES.filter(({ type }) => form.files[type]);
    for (const [i, { type, label }] of uploads.entries()) {
      busy.value = `Uploading the ${label} (${i + 1} of ${uploads.length})…`;
      try {
        await uploadLectureMedia(props.courseId, created[0].id, type, form.files[type]!);
      } catch (err) {
        // The rest would most likely fail the same way (no worker, or a live lecture).
        problem = mediaUploadErrorMessage(err, label);
        break;
      }
    }
  }

  busy.value = "";
  emit("created", created, problem);
}
</script>

<template>
  <section
    class="flex flex-col rounded border bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800"
    aria-labelledby="create-lecture-heading"
  >
    <header class="flex items-center justify-between gap-2 border-b px-4 py-3 dark:border-gray-700">
      <h3 id="create-lecture-heading" class="text-1 text-base font-semibold">Create a lecture</h3>
      <button
        type="button"
        class="text-5 hover:text-1 px-1"
        aria-label="Close"
        :disabled="!!busy"
        @click="emit('close')"
      >
        <i class="fas fa-xmark"></i>
      </button>
    </header>

    <ol class="text-5 flex flex-wrap items-center gap-x-4 gap-y-1 border-b px-4 py-2 text-sm dark:border-gray-700" aria-label="Steps">
      <li
        v-for="(s, i) in steps"
        :key="s"
        class="flex items-center gap-2"
        :class="{ 'font-semibold text-blue-600 dark:text-blue-400': i === stepIndex }"
        :aria-current="i === stepIndex ? 'step' : undefined"
      >
        <span
          class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border text-xs"
          :class="i === stepIndex ? 'border-blue-600 dark:border-blue-400' : 'border-gray-400'"
        >
          {{ i + 1 }}
        </span>
        {{ STEP_TITLES[s] }}
      </li>
    </ol>

    <form class="flex flex-col gap-4 p-4" novalidate @submit.prevent="next">
      <fieldset :disabled="!!busy" class="min-w-0">
        <CreateLectureTypeStep v-if="step === 'type'" :form="form" />
        <CreateLectureDetailsStep
          v-else-if="step === 'details'"
          :form="form"
          :halls="halls"
          :can-choose-hall="canChooseHall"
          :course-chat-enabled="courseChatEnabled"
        />
        <CreateLectureFilesStep v-else :form="form" />
      </fieldset>

      <ul v-if="attempted && problems.length" class="text-sm text-red-600 dark:text-red-400" aria-label="Problems">
        <li v-for="p in problems" :key="p">{{ p }}</li>
      </ul>
      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>
      <p v-if="busy" class="text-3 flex items-center gap-2 text-sm" role="status">
        <i class="fas fa-circle-notch animate-spin"></i>{{ busy }}
      </p>

      <div class="flex flex-wrap items-center justify-end gap-2">
        <button
          type="button"
          class="tum-live-button-secondary tum-live-button mr-auto px-3 py-1 text-sm"
          :disabled="!!busy"
          @click="emit('close')"
        >
          Cancel
        </button>
        <button
          v-if="stepIndex > 0"
          type="button"
          class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm"
          :disabled="!!busy"
          @click="back"
        >
          Back
        </button>
        <button type="submit" class="tum-live-button-primary px-3 py-1 text-sm" :disabled="!!busy">
          {{ submitLabel }}
        </button>
      </div>
    </form>
  </section>
</template>
