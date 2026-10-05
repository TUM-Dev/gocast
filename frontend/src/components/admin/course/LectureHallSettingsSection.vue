<script setup lang="ts">
import { computed, ref, watch } from "vue";

import {
  CourseSourceMode,
  fetchLectureHallSettings,
  updateLectureHallSettings,
  type LectureHallSetting,
} from "@/lib/course-admin";
import { courseError } from "@/stores/course-admin";

/**
 * Per lecture hall the course has lectures in: which sources to stream and which
 * camera preset to start from. Hidden for a course without any, rather than showing
 * an empty box.
 *
 * A dropdown of presets instead of the old grid of camera snapshots: the names are
 * what people chose by, and the grid needed horizontal scrolling at any width.
 */
const props = defineProps<{ courseId: number }>();

const halls = ref<LectureHallSetting[]>([]);
/** The halls as saved, to tell whether anything changed. */
const saved = ref("");
const loaded = ref(false);
const error = ref("");
const saving = ref(false);
const savedNotice = ref(false);

const snapshot = (list: LectureHallSetting[]) =>
  JSON.stringify(list.map((h) => [h.lectureHallId, h.sourceMode, h.selectedPresetId]));

function take(list: LectureHallSetting[]): void {
  halls.value = list;
  saved.value = snapshot(list);
}

watch(
  () => props.courseId,
  async (id) => {
    loaded.value = false;
    try {
      take(await fetchLectureHallSettings(id));
      error.value = "";
    } catch (err) {
      error.value = courseError(err);
    } finally {
      loaded.value = true;
    }
  },
  { immediate: true },
);

const dirty = computed(() => snapshot(halls.value) !== saved.value);
watch(dirty, (d) => {
  if (d) savedNotice.value = false;
});

const sourceModes = [
  { value: CourseSourceMode.COMBINED, label: "Presentation & Camera" },
  { value: CourseSourceMode.PRESENTATION_ONLY, label: "Presentation only" },
  { value: CourseSourceMode.CAMERA_ONLY, label: "Camera only" },
];

/** The old page hid presets named "gelöscht" (deleted), unless one is chosen. */
function presetsOf(hall: LectureHallSetting) {
  return hall.presets.filter((p) => p.name !== "gelöscht" || p.presetId === hall.selectedPresetId);
}

function defaultPresetName(hall: LectureHallSetting): string {
  const preset = hall.presets.find((p) => p.isDefault);
  return preset ? `Hall default (${preset.name})` : "Hall default";
}

async function save(): Promise<void> {
  saving.value = true;
  error.value = "";
  try {
    take(await updateLectureHallSettings(props.courseId, halls.value));
    savedNotice.value = true;
  } catch (err) {
    error.value = courseError(err);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <section
    v-if="error || (loaded && halls.length)"
    class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800"
    aria-labelledby="cs-halls"
  >
    <header>
      <h2 id="cs-halls" class="text-1 text-lg font-semibold">Lecture Hall Settings</h2>
      <p class="text-5 text-sm">
        Which sources the course's lectures stream in each lecture hall, and the camera preset they start with.
      </p>
    </header>

    <p
      v-if="error"
      class="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
      role="alert"
    >
      {{ error }}
    </p>

    <div
      v-for="hall in halls"
      :key="hall.lectureHallId"
      class="grid items-end gap-3 border-t pt-3 text-sm sm:grid-cols-[1fr_1fr_1fr] dark:border-gray-800"
    >
      <p class="text-1 self-center font-semibold">{{ hall.lectureHallName }}</p>
      <div class="flex flex-col gap-1">
        <label :for="`cs-hall-${hall.lectureHallId}-source`" class="text-4 text-xs">Sources</label>
        <select
          :id="`cs-hall-${hall.lectureHallId}-source`"
          v-model.number="hall.sourceMode"
          class="tum-live-input"
        >
          <option v-for="mode in sourceModes" :key="mode.value" :value="mode.value">{{ mode.label }}</option>
        </select>
      </div>
      <div class="flex flex-col gap-1">
        <label :for="`cs-hall-${hall.lectureHallId}-preset`" class="text-4 text-xs">Camera preset</label>
        <select
          :id="`cs-hall-${hall.lectureHallId}-preset`"
          v-model.number="hall.selectedPresetId"
          class="tum-live-input"
          :disabled="!hall.presets.length"
        >
          <option :value="0">{{ defaultPresetName(hall) }}</option>
          <option v-for="preset in presetsOf(hall)" :key="preset.presetId" :value="preset.presetId">
            {{ preset.name }}
          </option>
        </select>
      </div>
    </div>

    <div class="flex items-center justify-end gap-3">
      <p v-if="savedNotice" class="text-sm text-green-700 dark:text-green-400" role="status">Saved.</p>
      <button
        type="button"
        class="tum-live-button-primary px-4 py-2 text-sm"
        :disabled="saving || !dirty"
        @click="save"
      >
        {{ saving ? "Saving…" : "Save Lecture Hall Settings" }}
      </button>
    </div>
  </section>
</template>
