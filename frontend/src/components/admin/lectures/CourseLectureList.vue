<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from "vue";

import LectureCard from "@/components/admin/lectures/LectureCard.vue";
import CreateLectureWizard from "@/components/admin/lectures/create/CreateLectureWizard.vue";
import {
  deleteLectures,
  fetchAdministeredCourses,
  fetchCourseLectures,
  lectureErrorMessage,
  lectureFromHash,
  loadSortAscending,
  saveSortAscending,
  seriesSize,
  sortLectures,
  updateLecturesLectureHall,
  type AdministeredCourse,
  type CourseLecture,
} from "@/lib/course-lectures";
import { SELF_STREAMED, fetchScheduleLectureHalls, type ScheduleLectureHall } from "@/lib/schedule";
import { can } from "@/lib/settings";
import { useAuthStore } from "@/stores/auth";
import { useCourseAdminStore } from "@/stores/course-admin";

/**
 * A course's lectures for its administrators: sorting, selecting several to delete
 * or (for server administrators) to move to another hall, and each lecture's card.
 * Needs nothing but the course, so it renders the same inside whichever layout holds
 * it.
 */
const props = defineProps<{
  courseId: number;
  /** For the stream key and the watch links; empty until the course has loaded. */
  courseSlug: string;
}>();

const lectures = ref<CourseLecture[]>([]);
const loading = ref(true);
const error = ref("");
const status = ref("");

const auth = useAuthStore();
/**
 * Halls are shared across courses, so only server administrators move lectures
 * between them; the server refuses anyone else. Offering the control by the same
 * permission keeps the page from showing what would fail.
 */
const canChangeHall = computed(() => can(auth.user, "server.administer"));

/* Creating lectures. */

const courseStore = useCourseAdminStore();
const courseChatEnabled = computed(() =>
  courseStore.course?.id === props.courseId ? courseStore.course.chatEnabled : false,
);
const creating = ref(false);

/** Shows what the wizard created without reloading, and opens the first of it. */
async function onCreated(created: CourseLecture[], problem: string): Promise<void> {
  creating.value = false;
  const known = new Set(lectures.value.map((l) => l.id));
  lectures.value = [...lectures.value, ...created.filter((l) => !known.has(l.id))];
  const noun = created.length === 1 ? "lecture" : "lectures";
  status.value = `Created ${created.length} ${noun}.`;
  error.value = problem;
  const first = created[0];
  if (!first) return;
  expanded.add(first.id);
  await nextTick();
  document.getElementById(`lecture-${first.id}`)?.scrollIntoView({ block: "start" });
}

const halls = ref<ScheduleLectureHall[]>([]);
const courses = ref<AdministeredCourse[]>([]);
const targetCourses = computed(() => courses.value.filter((c) => c.id !== props.courseId));

const ascending = ref(loadSortAscending());
watch(ascending, saveSortAscending);
const sorted = computed(() => sortLectures(lectures.value, ascending.value));

const selected = reactive(new Set<number>());
const expanded = reactive(new Set<number>());

const allSelected = computed(
  () => lectures.value.length > 0 && lectures.value.every((l) => selected.has(l.id)),
);

function select(id: number, on: boolean): void {
  if (on) selected.add(id);
  else selected.delete(id);
}

function selectAll(on: boolean): void {
  selected.clear();
  if (on) lectures.value.forEach((l) => selected.add(l.id));
}

function toggle(id: number): void {
  if (expanded.has(id)) expanded.delete(id);
  else expanded.add(id);
}

async function reload(): Promise<void> {
  try {
    lectures.value = await fetchCourseLectures(props.courseId);
    error.value = "";
    // Drop what a deletion or a move took away.
    const ids = new Set(lectures.value.map((l) => l.id));
    [...selected].filter((id) => !ids.has(id)).forEach((id) => selected.delete(id));
    [...expanded].filter((id) => !ids.has(id)).forEach((id) => expanded.delete(id));
  } catch (err) {
    error.value = lectureErrorMessage(err);
  }
}

/** Opens and scrolls to the lecture `#lecture-<id>` names, as links into this page do. */
async function openFromHash(): Promise<void> {
  const id = lectureFromHash(window.location.hash);
  if (id === null || !lectures.value.some((l) => l.id === id)) return;
  expanded.add(id);
  await nextTick();
  document.getElementById(`lecture-${id}`)?.scrollIntoView({ block: "start" });
}

const bulkHall = ref(SELF_STREAMED);
/*
 * As v1: a lecture that has ended or been recorded no longer streams from anywhere,
 * so moving it is refused here rather than silently doing nothing useful.
 */
const bulkHallBlocked = computed(() =>
  lectures.value.some((l) => selected.has(l.id) && (l.past || l.recording)),
);

async function moveSelected(): Promise<void> {
  const ids = [...selected];
  if (!ids.length || bulkHallBlocked.value) return;
  const noun = ids.length === 1 ? "lecture" : "lectures";
  const name =
    bulkHall.value === SELF_STREAMED
      ? "self-streaming"
      : (halls.value.find((h) => h.id === bulkHall.value)?.name ?? "the hall");
  status.value = "";
  try {
    await updateLecturesLectureHall(props.courseId, ids, bulkHall.value);
    status.value = `Moved ${ids.length} ${noun} to ${name}.`;
    selected.clear();
  } catch (err) {
    error.value = lectureErrorMessage(err);
  }
  await reload();
}

async function deleteSelected(): Promise<void> {
  const ids = [...selected];
  if (!ids.length) return;
  const noun = ids.length === 1 ? "lecture" : "lectures";
  if (!window.confirm(`Delete ${ids.length} ${noun}? This includes any recordings.`)) return;
  status.value = "";
  try {
    await deleteLectures(props.courseId, ids);
    status.value = `Deleted ${ids.length} ${noun}.`;
    selected.clear();
  } catch (err) {
    error.value = lectureErrorMessage(err);
  }
  await reload();
}

watch(
  () => props.courseId,
  async () => {
    loading.value = true;
    selected.clear();
    creating.value = false;
    expanded.clear();
    // Neither is needed to show the list, so a failure leaves the editors without
    // hall names or copy targets rather than the page without lectures.
    fetchScheduleLectureHalls()
      .then((found) => (halls.value = found))
      .catch(() => (halls.value = []));
    fetchAdministeredCourses()
      .then((found) => (courses.value = found))
      .catch(() => (courses.value = []));
    await reload();
    loading.value = false;
    await openFromHash();
  },
  { immediate: true },
);
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex flex-wrap items-center gap-3">
        <label class="text-3 flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            :checked="allSelected"
            :disabled="!lectures.length"
            @change="selectAll(($event.target as HTMLInputElement).checked)"
          />
          Select all
        </label>
        <button
          type="button"
          class="rounded border border-red-500 px-3 py-1 text-sm text-red-600 hover:bg-red-500 hover:text-white disabled:opacity-50 disabled:hover:bg-transparent disabled:hover:text-red-600 dark:text-red-400"
          :disabled="selected.size === 0"
          @click="deleteSelected"
        >
          Delete {{ selected.size }} {{ selected.size === 1 ? "lecture" : "lectures" }}
        </button>
        <form v-if="canChangeHall" class="flex flex-wrap items-center gap-2" @submit.prevent="moveSelected">
          <label for="lectures-bulk-hall" class="text-3 text-sm">Move selected to</label>
          <select
            id="lectures-bulk-hall"
            v-model.number="bulkHall"
            class="tum-live-input py-1 text-sm"
            :disabled="selected.size === 0 || bulkHallBlocked"
          >
            <option :value="SELF_STREAMED">Self-streaming</option>
            <option v-for="hall in halls" :key="hall.id" :value="hall.id">{{ hall.name }}</option>
          </select>
          <button
            type="submit"
            class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm"
            :disabled="selected.size === 0 || bulkHallBlocked"
            :title="bulkHallBlocked ? 'Lectures that have ended or been recorded keep their hall.' : undefined"
          >
            Move
          </button>
        </form>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm"
          title="Change the order"
          @click="ascending = !ascending"
        >
          <i class="fas mr-1" :class="ascending ? 'fa-arrow-up' : 'fa-arrow-down'"></i>
          {{ ascending ? "Oldest first" : "Newest first" }}
        </button>
        <button
          type="button"
          class="tum-live-button-primary px-3 py-1 text-sm"
          :aria-expanded="creating"
          @click="creating = !creating"
        >
          <i class="fas fa-plus mr-1"></i>Create lecture
        </button>
      </div>
    </div>

    <CreateLectureWizard
      v-if="creating"
      :course-id="courseId"
      :halls="halls"
      :can-choose-hall="canChangeHall"
      :course-chat-enabled="courseChatEnabled"
      @created="onCreated"
      @close="creating = false"
    />

    <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>
    <!-- Beside an error rather than instead of it: a lecture created whose upload failed is both. -->
    <p v-if="status" class="text-5 text-sm" role="status">{{ status }}</p>

    <p v-if="loading" class="text-5 text-sm">Loading lectures…</p>
    <p v-else-if="!lectures.length && !error" class="text-5 text-sm">This course has no lectures yet.</p>

    <ul class="flex flex-col gap-3" aria-label="Lectures">
      <LectureCard
        v-for="lecture in sorted"
        :key="lecture.id"
        :lecture="lecture"
        :selected="selected.has(lecture.id)"
        :expanded="expanded.has(lecture.id)"
        :series-count="seriesSize(lectures, lecture)"
        :halls="halls"
        :can-change-hall="canChangeHall"
        :course-id="courseId"
        :course-slug="courseSlug"
        :target-courses="targetCourses"
        @update:selected="(on) => select(lecture.id, on)"
        @toggle="toggle(lecture.id)"
        @changed="reload"
      />
    </ul>
  </div>
</template>
