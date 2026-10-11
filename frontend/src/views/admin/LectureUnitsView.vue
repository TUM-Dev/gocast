<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import UnitPlayer from "@/components/admin/lectures/units/UnitPlayer.vue";
import {
  checkUnit,
  createLectureUnit,
  deleteLectureUnit,
  fetchLectureUnits,
  formatTimecode,
  parseTimecode,
  renderDescription,
  sortUnits,
  unitsErrorMessage,
  type LectureUnit,
} from "@/lib/lecture-units";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Where a lecturer cuts a recording into named units. The server refuses anyone who
 * does not administer the course before serving this page, and the three RPCs refuse
 * them again -- and also a lecture that is not the course's.
 */
const auth = useAuthStore();
const route = useRoute();

const courseId = computed(() => Number(route.params.courseID));
const streamId = computed(() => Number(route.params.streamID));

const lectureName = ref("");
const playlistUrl = ref("");
const serverDuration = ref(0);
// What the player measured, for a recording whose length the server did not record.
const playerDuration = ref(0);
const units = ref<LectureUnit[]>([]);
const loading = ref(true);
const error = ref("");

const player = ref<InstanceType<typeof UnitPlayer> | null>(null);

const duration = computed(() => serverDuration.value || playerDuration.value);

const draft = ref({ title: "", description: "", from: "00:00:00", to: "00:00:00" });
const formProblem = ref("");
const saving = ref(false);
const showForm = ref(false);
const showPreview = ref(false);
const preview = computed(() => renderDescription(draft.value.description));

// Two-step delete: the first click arms the button for that unit.
const armed = ref<number | null>(null);

async function load(course: number, stream: number): Promise<void> {
  loading.value = true;
  try {
    const res = await fetchLectureUnits(course, stream);
    lectureName.value = res.lectureName;
    playlistUrl.value = res.playlistUrl;
    serverDuration.value = res.durationSeconds;
    units.value = sortUnits(res.units);
    error.value = "";
  } catch (err) {
    error.value = unitsErrorMessage(err);
  } finally {
    loading.value = false;
  }
}

function openForm(): void {
  // Starts where the recording is playing and runs to its end, like the old slider's
  // two handles at either edge.
  draft.value.from = formatTimecode(player.value?.position() ?? 0);
  draft.value.to = formatTimecode(duration.value * 1000);
  formProblem.value = "";
  showForm.value = true;
}

function useNow(field: "from" | "to"): void {
  draft.value[field] = formatTimecode(player.value?.position() ?? 0);
}

/** The range inputs work in whole seconds; the text fields keep the timecode. */
function seconds(field: "from" | "to"): number {
  return Math.floor((parseTimecode(draft.value[field]) ?? 0) / 1000);
}

function slide(field: "from" | "to", event: Event): void {
  const value = Number((event.target as HTMLInputElement).value);
  draft.value[field] = formatTimecode(value * 1000);
  // As the old page did, show the frame the handle is on.
  player.value?.seek(value * 1000);
}

async function add(): Promise<void> {
  const checked = checkUnit(draft.value, duration.value);
  if (!checked.ok) {
    formProblem.value = checked.problem;
    return;
  }
  formProblem.value = "";
  saving.value = true;
  try {
    const created = await createLectureUnit(courseId.value, streamId.value, checked.unit);
    units.value = sortUnits([...units.value, created]);
    draft.value = { title: "", description: "", from: "00:00:00", to: "00:00:00" };
    showForm.value = false;
    showPreview.value = false;
    error.value = "";
  } catch (err) {
    formProblem.value = unitsErrorMessage(err);
  } finally {
    saving.value = false;
  }
}

async function remove(unit: LectureUnit): Promise<void> {
  if (armed.value !== unit.id) {
    armed.value = unit.id;
    return;
  }
  armed.value = null;
  try {
    await deleteLectureUnit(courseId.value, streamId.value, unit.id);
    units.value = units.value.filter((u) => u.id !== unit.id);
    error.value = "";
  } catch (err) {
    error.value = unitsErrorMessage(err);
  }
}

watch(
  [courseId, streamId],
  async ([course, stream]) => {
    const user = await auth.load().catch(() => null);
    if (!user) {
      redirectToLogin();
      return;
    }
    await load(course, stream);
  },
  { immediate: true },
);
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-6">
      <header>
        <h1 class="text-1 text-2xl font-bold">Lecture Units</h1>
        <p v-if="!loading && !error" class="text-3">
          <RouterLink :to="`/admin/courses/${courseId}/lectures`" class="hover:underline">Back to the lectures</RouterLink>
          <template v-if="lectureName"> · {{ lectureName }}</template>
        </p>
      </header>

      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>
      <p v-if="loading" class="text-5 text-sm">Loading units…</p>

      <template v-else-if="!error || units.length || lectureName">
        <div v-if="playlistUrl">
          <p class="text-5 mb-1 text-sm">Reference video</p>
          <UnitPlayer ref="player" :src="playlistUrl" @duration="playerDuration = $event" />
        </div>
        <p v-else class="text-5 text-sm">
          This lecture has no recording yet. You can still add units by entering their times.
        </p>

        <div>
          <h2 class="text-1 mb-2 text-lg font-semibold">Units</h2>
          <p v-if="!units.length" class="text-5 text-sm">This lecture has no units yet.</p>
          <ul v-else class="flex flex-col divide-y dark:divide-gray-700">
            <li v-for="unit in units" :key="unit.id" class="flex flex-col gap-1 py-2" :data-unit="unit.id">
              <div class="flex items-start gap-3">
                <div class="min-w-0 grow">
                  <p class="text-1 break-words font-semibold">{{ unit.title }}</p>
                  <p class="text-5 text-sm">
                    <button
                      v-if="playlistUrl"
                      type="button"
                      class="hover:underline"
                      :aria-label="`Jump to ${formatTimecode(unit.fromMs)}`"
                      @click="player?.seek(unit.fromMs)"
                    >
                      {{ formatTimecode(unit.fromMs) }}
                    </button>
                    <template v-else>{{ formatTimecode(unit.fromMs) }}</template>
                    – {{ formatTimecode(unit.toMs) }}
                  </p>
                </div>
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm"
                    :aria-label="armed === unit.id ? `Confirm deleting ${unit.title}` : `Delete ${unit.title}`"
                    @click="remove(unit)"
                  >
                    <i class="fas fa-trash mr-1"></i>{{ armed === unit.id ? "Really delete?" : "Delete" }}
                  </button>
                  <button
                    v-if="armed === unit.id"
                    type="button"
                    class="tum-live-button-tertiary px-3 py-1 text-sm"
                    @click="armed = null"
                  >
                    Keep
                  </button>
                </div>
              </div>
              <!-- Sanitized by DOMPurify in renderDescription. -->
              <div
                v-if="unit.description"
                class="lecture-description-preview text-3 text-sm"
                v-html="renderDescription(unit.description)"
              ></div>
            </li>
          </ul>
        </div>

        <div>
          <button v-if="!showForm" type="button" class="tum-live-button-primary px-4 py-2" @click="openForm">
            New Unit
          </button>
          <form v-else class="flex flex-col gap-3 border-t pt-3 dark:border-gray-700" @submit.prevent="add">
            <h2 class="text-1 text-lg font-semibold">New unit</h2>
            <label class="flex flex-col gap-1 text-sm">
              <span class="text-3">Title. Keep it short, perhaps one word and a number.</span>
              <input v-model="draft.title" type="text" placeholder="Introduction" class="tum-live-input" />
            </label>
            <div class="flex flex-col gap-1 text-sm">
              <div class="flex items-center justify-between">
                <label for="unit-description" class="text-3">Description</label>
                <button type="button" class="text-5 text-xs hover:underline" @click="showPreview = !showPreview">
                  {{ showPreview ? "Edit" : "Preview" }}
                </button>
              </div>
              <textarea
                v-show="!showPreview"
                id="unit-description"
                v-model="draft.description"
                rows="4"
                placeholder="You can use Markdown."
                class="tum-live-input"
              ></textarea>
              <template v-if="showPreview">
                <!-- Sanitized by DOMPurify in renderDescription. -->
                <div
                  v-if="preview"
                  class="lecture-description-preview text-3 min-h-16 rounded border p-2 dark:border-gray-700"
                  v-html="preview"
                ></div>
                <p v-else class="text-5 rounded border p-2 dark:border-gray-700">Nothing to preview.</p>
              </template>
            </div>

            <div class="grid gap-3 sm:grid-cols-2">
              <div v-for="field in ['from', 'to'] as const" :key="field" class="flex flex-col gap-1 text-sm">
                <label :for="`unit-${field}`" class="text-3">{{ field === "from" ? "Start" : "End" }}</label>
                <div class="flex gap-2">
                  <input
                    :id="`unit-${field}`"
                    v-model="draft[field]"
                    type="text"
                    inputmode="numeric"
                    placeholder="00:00:00"
                    class="tum-live-input w-32"
                  />
                  <button
                    v-if="playlistUrl"
                    type="button"
                    class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm"
                    @click="useNow(field)"
                  >
                    Use current position
                  </button>
                </div>
                <input
                  v-if="duration"
                  type="range"
                  min="0"
                  :max="duration"
                  :value="seconds(field)"
                  :aria-label="`${field === 'from' ? 'Start' : 'End'} slider`"
                  @input="slide(field, $event)"
                />
              </div>
            </div>

            <p v-if="formProblem" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ formProblem }}</p>
            <div class="flex gap-2">
              <button type="submit" class="tum-live-button-primary px-4 py-2" :disabled="saving">Save Unit</button>
              <button type="button" class="tum-live-button-tertiary px-4 py-2" @click="showForm = false">Cancel</button>
            </div>
          </form>
        </div>
      </template>
    </section>
  </AdminLayout>
</template>
