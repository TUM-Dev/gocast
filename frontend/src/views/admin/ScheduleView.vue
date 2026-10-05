<script setup lang="ts">
import {
  Calendar,
  type CalendarOptions,
  type EventClickArg,
  type EventSourceFuncArg,
} from "@fullcalendar/core";
import timeGridPlugin from "@fullcalendar/timegrid";
import { computed, onMounted, onUnmounted, reactive, ref, watch } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  SELF_STREAMED,
  fetchSchedule,
  fetchScheduleLectureHalls,
  updateLecture,
  type ScheduleLectureHall,
  type ScheduledLecture,
} from "@/lib/schedule";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * The lectures of the courses the caller administers, on a day or week calendar --
 * the page /admin has always been. Clicking a lecture opens it for renaming and a
 * new description; everything else about it is on its course's page.
 */
const auth = useAuthStore();

const calendarEl = ref<HTMLElement | null>(null);
let calendar: Calendar | null = null;

const halls = ref<ScheduleLectureHall[]>([]);
/** Hall IDs shown, SELF_STREAMED included; every hall until someone narrows it. */
const shown = ref<Set<number>>(new Set([SELF_STREAMED]));
const showAll = ref(true);
const filterOpen = ref(false);

const loadError = ref("");

const selected = ref<ScheduledLecture | null>(null);
const edit = reactive({ name: "", description: "" });
const saving = ref(false);
const saveError = ref("");
const saved = ref("");

const hallOptions = computed(() => [
  { id: SELF_STREAMED, name: "Self-streaming" },
  ...halls.value,
]);

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to view the schedule.";
    if (err.status === 404) return "This lecture no longer exists or you do not administer it.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

function toggleHall(id: number, on: boolean): void {
  const next = new Set(showAll.value ? hallOptions.value.map((h) => h.id) : shown.value);
  if (on) next.add(id);
  else next.delete(id);
  showAll.value = next.size === hallOptions.value.length;
  shown.value = next;
}

function toggleAll(on: boolean): void {
  showAll.value = on;
  shown.value = on ? new Set(hallOptions.value.map((h) => h.id)) : new Set();
}

const isShown = (id: number) => showAll.value || shown.value.has(id);

watch([showAll, shown], () => calendar?.refetchEvents());

async function events(info: EventSourceFuncArg) {
  const lectures = await fetchSchedule(info.start, info.end, showAll.value ? "all" : [...shown.value]);
  loadError.value = "";
  return lectures.map((lecture) => ({
    id: String(lecture.streamId),
    title: lecture.courseName,
    start: lecture.start,
    end: lecture.end,
    extendedProps: { lecture },
  }));
}

function open(arg: EventClickArg): void {
  const lecture = arg.event.extendedProps.lecture as ScheduledLecture;
  selected.value = lecture;
  edit.name = lecture.name;
  edit.description = lecture.description;
  saveError.value = "";
  saved.value = "";
}

function close(): void {
  selected.value = null;
}

async function save(field: "name" | "description"): Promise<void> {
  const lecture = selected.value;
  if (!lecture) return;
  saving.value = true;
  saveError.value = "";
  saved.value = "";
  try {
    await updateLecture(lecture.courseId, lecture.streamId, { [field]: edit[field] });
    lecture[field] = field === "name" ? edit.name.trim() : edit.description;
    saved.value = field === "name" ? "Title saved." : "Description saved.";
  } catch (err) {
    saveError.value = message(err);
  } finally {
    saving.value = false;
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === "Escape" && selected.value) close();
}

onMounted(async () => {
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
    return;
  }

  fetchScheduleLectureHalls()
    .then((found) => (halls.value = found))
    .catch((err) => (loadError.value = message(err)));

  if (!calendarEl.value) return;
  // allDaySlot belongs to the timegrid plugin, whose declaration merging does not
  // reach CalendarOptions under this project's module resolution.
  const options: CalendarOptions & { allDaySlot: boolean } = {
    plugins: [timeGridPlugin],
    initialView: "timeGridDay",
    headerToolbar: { left: "prev,next today", center: "title", right: "timeGridDay,timeGridWeek" },
    nowIndicator: true,
    firstDay: 1,
    height: "75vh",
    allDaySlot: false,
    events: (info, success, failure) => {
      events(info)
        .then(success)
        .catch((err) => {
          loadError.value = message(err);
          failure(err);
        });
    },
    // Where it is held, after the time, as the old page showed it. Text nodes, not
    // HTML: course and hall names are user input.
    eventDidMount: ({ el, event }) => {
      const lecture = event.extendedProps.lecture as ScheduledLecture;
      el.title = lecture.lectureHallName
        ? `${lecture.courseName} · ${lecture.lectureHallName}`
        : lecture.courseName;
      const time = el.querySelector(".fc-event-time");
      if (time && lecture.lectureHallName) time.append(` · ${lecture.lectureHallName}`);
    },
    eventClick: open,
  };
  calendar = new Calendar(calendarEl.value, options);
  calendar.render();
  window.addEventListener("keydown", onKeydown);
});

onUnmounted(() => {
  calendar?.destroy();
  calendar = null;
  window.removeEventListener("keydown", onKeydown);
});
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-6xl flex-col gap-4">
      <header class="flex flex-wrap items-center justify-between gap-2">
        <h1 class="text-1 text-2xl font-bold">Schedule</h1>
        <div class="relative">
          <button
            type="button"
            class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm"
            :aria-expanded="filterOpen"
            aria-controls="schedule-halls"
            @click="filterOpen = !filterOpen"
          >
            Lecture halls
          </button>
          <fieldset
            v-show="filterOpen"
            id="schedule-halls"
            class="text-3 absolute right-0 z-20 mt-2 max-h-96 w-64 overflow-y-auto rounded-lg border bg-white p-3 text-sm shadow dark:border-gray-700 dark:bg-gray-800"
          >
            <legend class="sr-only">Lecture halls to show</legend>
            <label class="mb-2 block font-semibold">
              <input type="checkbox" :checked="showAll" @change="toggleAll(($event.target as HTMLInputElement).checked)" />
              All
            </label>
            <label v-for="hall in hallOptions" :key="hall.id" class="block">
              <input
                type="checkbox"
                :checked="isShown(hall.id)"
                @change="toggleHall(hall.id, ($event.target as HTMLInputElement).checked)"
              />
              {{ hall.name }}
            </label>
          </fieldset>
        </div>
      </header>

      <p v-if="loadError" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
        {{ loadError }}
      </p>

      <div ref="calendarEl" class="schedule-calendar text-3"></div>
    </section>

    <div
      v-if="selected"
      class="fixed inset-0 z-40 flex items-center justify-center bg-black/30 p-4 backdrop-blur-xs"
      @click.self="close"
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="schedule-lecture-course"
        class="w-full max-w-lg rounded-lg border bg-white p-4 shadow-lg dark:border-gray-700 dark:bg-gray-800"
      >
        <div class="flex items-start gap-2">
          <h2 id="schedule-lecture-course" class="text-1 grow text-lg font-semibold">
            {{ selected.courseName }}
          </h2>
          <button type="button" class="text-4 hover:text-1" aria-label="Close" @click="close">
            <i class="fas fa-times"></i>
          </button>
        </div>
        <p class="text-3 text-sm">
          {{ selected.start.toLocaleString() }}
          <template v-if="selected.lectureHallName"> · {{ selected.lectureHallName }}</template>
        </p>

        <p
          v-if="saveError"
          class="mt-2 rounded-lg bg-danger/25 px-2 py-1 text-sm"
          role="alert"
        >
          {{ saveError }}
        </p>
        <p v-else-if="saved" class="text-5 mt-2 text-sm" role="status">{{ saved }}</p>

        <form class="mt-3 flex flex-col gap-1 text-sm" @submit.prevent="save('name')">
          <label for="schedule-lecture-name" class="text-2">Lecture title</label>
          <div class="flex gap-2">
            <input
              id="schedule-lecture-name"
              v-model="edit.name"
              type="text"
              autocomplete="off"
              placeholder="Lecture 2: Dark-Patterns I"
              class="tum-live-input grow"
            />
            <button
              type="submit"
              class="tum-live-button-primary px-3 text-sm"
              :disabled="saving || edit.name.trim() === selected.name"
            >
              Save
            </button>
          </div>
        </form>

        <form class="mt-3 flex flex-col gap-1 text-sm" @submit.prevent="save('description')">
          <label for="schedule-lecture-description" class="text-2">Description</label>
          <textarea
            id="schedule-lecture-description"
            v-model="edit.description"
            rows="3"
            placeholder="Add a nice description, links, and more. You can use Markdown."
            class="tum-live-input"
          ></textarea>
          <div class="flex justify-end">
            <button
              type="submit"
              class="tum-live-button-primary px-3 py-1 text-sm"
              :disabled="saving || edit.description === selected.description"
            >
              Save
            </button>
          </div>
        </form>

        <p class="text-5 mt-3 text-xs">
          Viewers watching right now see a new title or description when they next load the page.
        </p>
        <!-- The course's lecture list opens the card the fragment names. -->
        <a
          class="text-3 hover:text-1 mt-2 inline-block text-sm"
          :href="`/admin/courses/${selected.courseId}/lectures#lecture-${selected.streamId}`"
        >
          Edit everything else on the course's lectures page <i class="fas fa-external-link-alt"></i>
        </a>
      </div>
    </div>
  </AdminLayout>
</template>

<style>
/*
 * FullCalendar injects its own stylesheet, unlayered, and renders its markup outside
 * Vue's reach, so these cannot be scoped. Carried over from web/assets/css/main.css.
 */
.dark .schedule-calendar table,
.dark .schedule-calendar td,
.dark .schedule-calendar th,
.dark .schedule-calendar .fc-theme-standard td,
.dark .schedule-calendar .fc-theme-standard th {
  border-color: #474747;
}

.schedule-calendar .fc-v-event {
  background-color: #3070b3;
  cursor: pointer;
}

.schedule-calendar .fc-timegrid-event {
  box-shadow: 0 0 0 1px #ffffff60;
}
</style>
