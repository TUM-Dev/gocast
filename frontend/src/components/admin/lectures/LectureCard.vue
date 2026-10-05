<script setup lang="ts">
import { computed } from "vue";

import LectureEditor from "@/components/admin/lectures/LectureEditor.vue";
import TranscodingProgress from "@/components/admin/lectures/content/TranscodingProgress.vue";
import {
  formatDuration,
  lectureBadges,
  lectureState,
  type AdministeredCourse,
  type BadgeTone,
  type CourseLecture,
  type LectureState,
} from "@/lib/course-lectures";
import type { ScheduleLectureHall } from "@/lib/schedule";

/**
 * One lecture of a course's list: a summary that opens into its editor. Its element
 * ID, `lecture-<id>`, is the fragment that opens it on load.
 */
const props = defineProps<{
  lecture: CourseLecture;
  selected: boolean;
  expanded: boolean;
  seriesCount: number;
  halls: ScheduleLectureHall[];
  /** Server administrators only; see CourseLectureList. */
  canChangeHall: boolean;
  courseId: number;
  courseSlug: string;
  targetCourses: AdministeredCourse[];
}>();

const emit = defineEmits<{
  "update:selected": [value: boolean];
  toggle: [];
  changed: [];
}>();

const state = computed(() => lectureState(props.lecture));
const badges = computed(() => lectureBadges(props.lecture));
const duration = computed(() => formatDuration(props.lecture.durationSeconds));

/** The coloured strip along the top, as the old card's top border. */
const STRIP: Record<LectureState, string> = {
  live: "bg-danger",
  converting: "bg-info",
  recording: "bg-success",
  past: "bg-warn",
  planned: "bg-info",
};

const TONE: Record<BadgeTone, string> = {
  danger: "bg-danger text-white",
  success: "bg-success text-black",
  warn: "bg-warn text-black",
  info: "bg-info text-white",
  muted: "bg-gray-200 text-gray-700 dark:bg-gray-700 dark:text-gray-200",
};

const date = computed(() =>
  props.lecture.start.toLocaleDateString(undefined, {
    weekday: "short",
    year: "numeric",
    month: "short",
    day: "numeric",
  }),
);
const time = (d: Date) => d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });

const editorId = computed(() => `lecture-${props.lecture.id}-editor`);
const watchLink = computed(() =>
  props.courseSlug ? `/w/${props.courseSlug}/${props.lecture.id}` : "",
);
</script>

<template>
  <li
    :id="`lecture-${lecture.id}`"
    class="overflow-hidden rounded border bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800"
  >
    <div class="h-1" :class="STRIP[state]"></div>
    <div class="flex flex-wrap items-start gap-3 p-3">
      <input
        type="checkbox"
        class="mt-1"
        :checked="selected"
        :aria-label="`Select ${lecture.name || 'this lecture'}`"
        @change="emit('update:selected', ($event.target as HTMLInputElement).checked)"
      />
      <div class="flex min-w-0 grow basis-60 flex-col gap-1">
        <p class="text-3 text-sm font-semibold">
          <a v-if="watchLink" :href="watchLink" class="hover:underline">{{ date }}</a>
          <template v-else>{{ date }}</template>
          <span class="text-5 font-normal"> · {{ time(lecture.start) }} – {{ time(lecture.end) }}</span>
        </p>
        <h2 class="text-1 break-words text-base font-semibold">
          <span v-if="lecture.name">{{ lecture.name }}</span>
          <span v-else class="text-5 italic">Untitled lecture</span>
        </h2>
        <p class="text-5 flex flex-wrap gap-x-3 text-sm">
          <span><i class="fas fa-location-pin mr-1"></i>{{ lecture.lectureHallName || "Self-streamed" }}</span>
          <span v-if="duration"><i class="fas fa-clock mr-1"></i>{{ duration }}</span>
          <span v-if="lecture.vodVersions.length">
            <i class="fas fa-film mr-1"></i>{{ lecture.vodVersions.join(", ") }}
          </span>
          <span v-if="seriesCount"><i class="fas fa-layer-group mr-1"></i>Series of {{ seriesCount }}</span>
        </p>
        <TranscodingProgress
          :course-id="courseId"
          :lecture-id="lecture.id"
          :converting="lecture.converting"
          :progresses="lecture.transcodingProgresses"
          @done="emit('changed')"
        />
      </div>
      <div class="flex flex-col items-end gap-2">
        <ul class="flex flex-wrap justify-end gap-1" aria-label="State">
          <li
            v-for="badge in badges"
            :key="badge.key"
            class="rounded-full px-2 py-0.5 text-xs font-semibold"
            :class="TONE[badge.tone]"
          >
            <i v-if="badge.key === 'private'" class="fas fa-eye-slash mr-1"></i>{{ badge.label }}
          </li>
        </ul>
        <button
          type="button"
          class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm"
          :aria-expanded="expanded"
          :aria-controls="editorId"
          @click="emit('toggle')"
        >
          {{ expanded ? "Close" : "Edit" }}
        </button>
      </div>
    </div>
    <LectureEditor
      v-if="expanded"
      :id="editorId"
      :lecture="lecture"
      :series-count="seriesCount"
      :halls="halls"
      :can-change-hall="canChangeHall"
      :course-id="courseId"
      :course-slug="courseSlug"
      :target-courses="targetCourses"
      @changed="emit('changed')"
    />
  </li>
</template>
