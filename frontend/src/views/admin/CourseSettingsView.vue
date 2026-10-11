<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";

import CourseActionsSection from "@/components/admin/course/CourseActionsSection.vue";
import CourseAdminsSection from "@/components/admin/course/CourseAdminsSection.vue";
import CourseIntegrationGrantsSection from "@/components/admin/course/CourseIntegrationGrantsSection.vue";
import LectureHallSettingsSection from "@/components/admin/course/LectureHallSettingsSection.vue";
import {
  publicCoursePath,
  updateCourseSettings,
  type CourseAdmin,
  type CourseSettings,
  type CourseVisibility,
} from "@/lib/course-admin";
import { semesterLabel } from "@/lib/semesters";
import { courseError, useCourseAdminStore } from "@/stores/course-admin";

/**
 * The settings tab: what the old page's settings form edited, then its administrators,
 * lecture-hall sources and the copy and delete actions.
 *
 * Name, slug, semester, language and TUMOnline identifier are shown but not edited:
 * the old form never edited them either, and updateCourseSettings does not take them.
 *
 * Left out on purpose: the "Reload Enrollments" button, which the old handler no
 * longer recognised and so only saved the settings, and the dialog offering to
 * overwrite every lecture's chat setting, which never worked as described.
 */
const store = useCourseAdminStore();

// Rendered only once the layout has the course.
const course = computed(() => store.course as CourseAdmin);

const form = reactive<CourseSettings>({
  visibility: "public",
  vodEnabled: false,
  downloadsEnabled: false,
  chatEnabled: false,
  anonymousChatEnabled: false,
  moderatedChatEnabled: false,
  livePrivate: false,
  vodPrivate: false,
});

function reset(from: CourseAdmin): void {
  form.visibility = from.visibility;
  form.vodEnabled = from.vodEnabled;
  form.downloadsEnabled = from.downloadsEnabled;
  form.chatEnabled = from.chatEnabled;
  form.anonymousChatEnabled = from.anonymousChatEnabled;
  form.moderatedChatEnabled = from.moderatedChatEnabled;
  form.livePrivate = from.livePrivate;
  form.vodPrivate = from.vodPrivate;
}

watch(course, (c) => c && reset(c), { immediate: true });

/** Only what changed, so the audit log names what someone actually did. */
const changes = computed(() => {
  const out: Partial<CourseSettings> = {};
  for (const key of Object.keys(form) as (keyof CourseSettings)[]) {
    if (form[key] !== course.value[key]) {
      (out as Record<string, unknown>)[key] = form[key];
    }
  }
  return out;
});
const dirty = computed(() => Object.keys(changes.value).length > 0);

const saving = ref(false);
const error = ref("");
const saved = ref(false);

watch(
  () => ({ ...form }),
  () => {
    saved.value = false;
  },
);

async function save(): Promise<void> {
  saving.value = true;
  error.value = "";
  try {
    const updated = await updateCourseSettings(course.value.id, changes.value);
    store.replace(updated);
    saved.value = true;
  } catch (err) {
    error.value = courseError(err);
  } finally {
    saving.value = false;
  }
}

const hiddenLink = computed(() => `${window.location.origin}${publicCoursePath(course.value)}`);

const visibilities = computed<{ value: CourseVisibility; label: string; help: string; disabled?: boolean }[]>(
  () => [
    { value: "public", label: "Public", help: "Everyone can see this course." },
    {
      value: "enrolled",
      label: "Enrolled",
      help: course.value.tumOnlineIdentifier
        ? "Only students enrolled in TUMOnline can see this course."
        : "Only students enrolled in TUMOnline can see this course. Needs the course to be linked to TUMOnline.",
      // Without a TUMOnline course nobody can be enrolled, so nobody could see it.
      disabled: !course.value.tumOnlineIdentifier && course.value.visibility !== "enrolled",
    },
    { value: "loggedin", label: "Logged in", help: "Only users with a TUM account can see this course." },
    { value: "hidden", label: "Hidden", help: "Only visible to users with the link:" },
  ],
);

const languages: Record<string, string> = { de: "German", en: "English" };
</script>

<template>
  <div class="flex flex-col gap-6">
    <section class="rounded-lg border p-4 dark:border-gray-800" aria-labelledby="cs-course">
      <h2 id="cs-course" class="text-1 mb-3 text-lg font-semibold">Course</h2>
      <dl class="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[max-content_1fr]">
        <dt class="text-4">Name</dt>
        <dd class="text-2">{{ course.name }}</dd>
        <dt class="text-4">Slug</dt>
        <dd class="text-2 font-mono">{{ course.slug }}</dd>
        <dt class="text-4">Semester</dt>
        <dd class="text-2">{{ semesterLabel({ year: course.year, term: course.term }) }}</dd>
        <dt class="text-4">Language</dt>
        <dd class="text-2">{{ languages[course.language] ?? "Not specified" }}</dd>
        <dt class="text-4">TUMOnline</dt>
        <dd class="text-2">{{ course.tumOnlineIdentifier || "Not linked" }}</dd>
      </dl>
    </section>

    <form
      class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800"
      aria-labelledby="cs-settings"
      @submit.prevent="save"
    >
      <h2 id="cs-settings" class="text-1 text-lg font-semibold">Settings</h2>

      <p
        v-if="error"
        class="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
        role="alert"
      >
        {{ error }}
      </p>

      <fieldset class="flex flex-col gap-2 text-sm">
        <legend class="text-2 mb-1 font-semibold">Visibility</legend>
        <label
          v-for="option in visibilities"
          :key="option.value"
          class="flex items-start gap-2"
          :class="option.disabled ? 'opacity-60' : ''"
        >
          <input
            v-model="form.visibility"
            type="radio"
            name="visibility"
            class="mt-1"
            :value="option.value"
            :disabled="option.disabled"
          />
          <span>
            <span class="text-1 font-medium">{{ option.label }}:</span>
            <span class="text-3 ml-1">{{ option.help }}</span>
            <a
              v-if="option.value === 'hidden'"
              :href="publicCoursePath(course)"
              class="text-3 ml-1 break-all underline"
              >{{ hiddenLink }}</a
            >
          </span>
        </label>
      </fieldset>

      <fieldset class="flex flex-col gap-2 text-sm">
        <legend class="text-2 mb-1 font-semibold">Preferences</legend>
        <label class="flex items-start gap-2">
          <input v-model="form.vodEnabled" type="checkbox" class="mt-1" />
          <span
            ><span class="text-1">Enable VoD</span>
            <span class="text-5 block text-xs">Students can watch lectures after the livestream ended.</span></span
          >
        </label>
        <label class="flex items-start gap-2">
          <input v-model="form.downloadsEnabled" type="checkbox" class="mt-1" />
          <span
            ><span class="text-1">Enable downloads</span>
            <span class="text-5 block text-xs">Students can download lectures after the livestream ended.</span></span
          >
        </label>
        <label class="flex items-start gap-2">
          <input v-model="form.chatEnabled" type="checkbox" class="mt-1" />
          <span
            ><span class="text-1">Enable live chat</span>
            <span class="text-5 block text-xs">Students can use the chat during the lecture.</span></span
          >
        </label>
        <div class="flex flex-col gap-2 pl-6" :class="form.chatEnabled ? '' : 'opacity-60'">
          <label class="flex items-start gap-2">
            <input
              v-model="form.anonymousChatEnabled"
              type="checkbox"
              class="mt-1"
              :disabled="!form.chatEnabled"
            />
            <span
              ><span class="text-1">Allow anonymous messages</span>
              <span class="text-5 block text-xs">Students may send messages without their name.</span></span
            >
          </label>
          <label class="flex items-start gap-2">
            <input
              v-model="form.moderatedChatEnabled"
              type="checkbox"
              class="mt-1"
              :disabled="!form.chatEnabled"
            />
            <span
              ><span class="text-1">Moderate the chat</span>
              <span class="text-5 block text-xs"
                >Every message has to be approved by a moderator before others see it.</span
              ></span
            >
          </label>
          <p class="text-5 text-xs">
            Chat on recordings: {{ course.vodChatEnabled ? "on" : "off" }} (not configurable).
          </p>
        </div>
      </fieldset>

      <fieldset class="flex flex-col gap-2 text-sm">
        <legend class="text-2 mb-1 font-semibold">Advanced visibility</legend>
        <label class="flex items-center gap-2">
          <input v-model="form.livePrivate" type="checkbox" />
          <span class="text-1">Private livestreams</span>
        </label>
        <label class="flex items-center gap-2">
          <input v-model="form.vodPrivate" type="checkbox" />
          <span class="text-1">Private recordings after the livestream</span>
        </label>
      </fieldset>

      <div class="flex flex-wrap items-center justify-end gap-3">
        <p v-if="saved" class="text-sm text-green-700 dark:text-green-400" role="status">Settings saved.</p>
        <p v-else-if="dirty" class="text-5 text-sm">Unsaved changes</p>
        <button type="submit" class="tum-live-button-primary px-4 py-2 text-sm" :disabled="saving || !dirty">
          {{ saving ? "Saving…" : "Save Settings" }}
        </button>
      </div>
    </form>

    <CourseAdminsSection :course-id="course.id" />
    <CourseIntegrationGrantsSection :course-id="course.id" />
    <LectureHallSettingsSection :course-id="course.id" />
    <CourseActionsSection :course="course" />
  </div>
</template>
