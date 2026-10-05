<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  courseImportDepartments,
  importCourseImportCourses,
  searchCourseImportSchedule,
  type CourseImportCourseUI,
  type CourseImportResultUI,
} from "@/lib/course-import";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Importing courses from TUMonline: a three-step wizard, same shape as the legacy
 * Alpine page it replaces.
 *
 *   0. Search: pick a department, a date range and opt-in/opt-out, and search
 *      TUMonline's room schedule.
 *   1. Review: adjust which found courses (and which of their events) to import, and
 *      their slugs, before committing anything.
 *   2. Result: what was created and what failed, per course -- a course failing (a
 *      duplicate slug, an LDAP lookup miss) does not hide whether the others worked.
 */
const auth = useAuthStore();

type Step = "search" | "review" | "result";
const step = ref<Step>("search");
const loading = ref(false);
const error = ref("");

const currentYear = new Date().getFullYear();
const years = Array.from({ length: 6 }, (_, i) => currentYear - 1 + i);

const year = ref(currentYear);
const term = ref<"W" | "S">("W");
const departmentLabel = ref(courseImportDepartments[0].label);
const customDepartmentId = ref("");
const fromDate = ref("");
const toDate = ref("");
const optIn = ref<"Opt In" | "Opt Out">("Opt In");

const isCustomDepartment = computed(() => departmentLabel.value === "custom");
const departmentId = computed<number | null>(() => {
  if (isCustomDepartment.value) {
    const id = Number.parseInt(customDepartmentId.value, 10);
    return Number.isNaN(id) ? null : id;
  }
  return courseImportDepartments.find((d) => d.label === departmentLabel.value)?.departmentId ?? null;
});

const courses = ref<CourseImportCourseUI[]>([]);
const results = ref<CourseImportResultUI[]>([]);

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to import courses.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

onMounted(async () => {
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
  }
});

// Warns before leaving mid-review, same as the legacy page: TUMonline was just
// queried once per course found (to fetch contacts), and losing that on an accidental
// back-navigation means doing it all again.
function warnBeforeUnload(e: BeforeUnloadEvent): void {
  e.preventDefault();
  e.returnValue = "";
}
watch(step, (value) => {
  if (value === "review") {
    window.addEventListener("beforeunload", warnBeforeUnload);
  } else {
    window.removeEventListener("beforeunload", warnBeforeUnload);
  }
});
onUnmounted(() => window.removeEventListener("beforeunload", warnBeforeUnload));

async function search(): Promise<void> {
  error.value = "";
  if (!fromDate.value || !toDate.value) {
    error.value = "Both a start and an end date are required.";
    return;
  }
  if (departmentId.value === null) {
    error.value = "Enter a department id.";
    return;
  }

  loading.value = true;
  try {
    courses.value = await searchCourseImportSchedule(
      new Date(fromDate.value),
      new Date(toDate.value),
      departmentId.value,
    );
    step.value = "review";
  } catch (err) {
    error.value = message(err);
  } finally {
    loading.value = false;
  }
}

async function doImport(): Promise<void> {
  error.value = "";
  loading.value = true;
  try {
    results.value = await importCourseImportCourses(
      year.value,
      term.value,
      optIn.value === "Opt In",
      courses.value,
    );
    step.value = "result";
  } catch (err) {
    error.value = message(err);
  } finally {
    loading.value = false;
  }
}

function formatEventTime(event: { start: Date; end: Date }): string {
  const start = event.start.toISOString().slice(0, 16).replace("T", " ");
  const end = event.end.toISOString().slice(11, 16);
  return `${start} - ${end}`;
}
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-4xl flex-col gap-6">
      <h1 class="text-1 text-2xl font-bold">Course Import</h1>

      <Transition name="fade" mode="out-in">
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
          {{ error }}
        </p>
      </Transition>

      <p v-if="loading" class="text-5 text-center text-sm">
        <i class="fas fa-circle-notch animate-spin"></i> Loading…
      </p>

      <!-- Step 0: search -->
      <form
        v-if="step === 'search'"
        class="grid grid-cols-1 gap-4 rounded-lg border p-4 sm:grid-cols-2 dark:border-gray-800"
        @submit.prevent="search"
      >
        <label class="flex flex-col gap-1 text-sm">
          <span class="text-2">Year</span>
          <span class="text-4 text-xs"
            >The current year. For a winter semester, select the first of its two
            years.</span
          >
          <select v-model="year" class="tum-live-input">
            <option v-for="y in years" :key="y" :value="y">{{ y }}</option>
          </select>
        </label>
        <label class="flex flex-col gap-1 text-sm">
          <span class="text-2">Summer / Winter</span>
          <select v-model="term" class="tum-live-input">
            <option value="W">W</option>
            <option value="S">S</option>
          </select>
        </label>
        <label class="flex flex-col gap-1 text-sm">
          <span class="text-2">Department</span>
          <select v-model="departmentLabel" class="tum-live-input">
            <option v-for="d in courseImportDepartments" :key="d.label" :value="d.label">
              {{ d.label }}
            </option>
            <option value="custom">-- specify id --</option>
          </select>
        </label>
        <label v-if="isCustomDepartment" class="flex flex-col gap-1 text-sm">
          <span class="text-2">Department id</span>
          <input v-model="customDepartmentId" type="text" class="tum-live-input" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          <span class="text-2">Opt In / Opt Out</span>
          <span class="text-4 text-xs"
            >Opt In: a lecturer has to activate streaming. Opt Out: it starts on unless
            they decline.</span
          >
          <select v-model="optIn" class="tum-live-input">
            <option>Opt In</option>
            <option>Opt Out</option>
          </select>
        </label>
        <label class="flex flex-col gap-1 text-sm sm:col-span-2">
          <span class="text-2">Import events in this range</span>
          <div class="flex items-center gap-2">
            <input v-model="fromDate" type="date" class="tum-live-input" aria-label="From" />
            <span class="text-4">to</span>
            <input v-model="toDate" type="date" class="tum-live-input" aria-label="To" />
          </div>
        </label>
        <button
          type="submit"
          class="tum-live-input-submit tum-live-button-primary sm:col-span-2 px-4 py-3"
          :disabled="loading"
        >
          Start Import
        </button>
      </form>

      <!-- Step 1: review -->
      <div v-else-if="step === 'review'" class="flex flex-col gap-4">
        <p v-if="!loading && !courses.length" class="text-5 text-sm">
          TUMonline reported no courses in this range.
        </p>

        <div
          v-for="course in courses"
          :key="course.courseId"
          class="rounded-lg border p-4 dark:border-gray-800"
        >
          <h3 class="text-1 font-semibold">{{ course.title }}</h3>
          <label class="mt-1 flex items-center gap-2 text-sm">
            <input v-model="course.import" type="checkbox" class="w-auto" />
            <span class="text-3">Import this course</span>
          </label>

          <div v-if="course.import" class="mt-3 flex flex-col gap-3">
            <label class="flex items-center gap-2 text-sm">
              <span class="text-3">Slug:</span>
              <input v-model="course.slug" type="text" class="tum-live-input w-auto" />
              <span v-if="course.slug.length < 3" class="text-warn text-xs">short!</span>
            </label>

            <table v-if="course.contacts.length" class="w-full table-auto text-left text-sm">
              <tbody>
                <tr
                  v-for="(contact, i) in course.contacts"
                  :key="i"
                  :class="contact.mainContact ? 'text-green-500' : 'text-3'"
                >
                  <td class="py-1 pr-2">
                    <label class="flex items-center">
                      <i v-if="contact.mainContact" class="fas fa-envelope mr-1"></i>
                      <input v-model="contact.mainContact" type="checkbox" class="w-auto" />
                    </label>
                  </td>
                  <td class="pr-2">{{ contact.role }}:</td>
                  <td class="pr-2">{{ contact.firstName }} {{ contact.lastName }}</td>
                  <td>
                    <a :href="`mailto:${contact.email}`" class="hover:underline">{{
                      contact.email
                    }}</a>
                  </td>
                </tr>
              </tbody>
            </table>

            <div
              v-for="(event, i) in course.events"
              :key="i"
              class="flex flex-wrap items-center gap-3 border-t py-1 text-sm dark:border-gray-800"
            >
              <label class="flex items-center gap-2">
                <input v-model="event.import" type="checkbox" class="w-auto" />
                <span class="text-2">{{ formatEventTime(event) }}</span>
              </label>
              <span class="text-5" :title="event.roomName">
                <i class="fas fa-location-arrow mr-1"></i>{{ event.roomName }}
              </span>
              <span v-if="event.comment" class="text-warn" :title="event.comment">
                <i class="fas fa-info-circle mr-1"></i>{{ event.comment }}
              </span>
            </div>
          </div>
        </div>

        <button
          type="button"
          class="tum-live-input-submit tum-live-button-primary px-4 py-3"
          :disabled="loading || !courses.length"
          @click="doImport"
        >
          Import
        </button>
      </div>

      <!-- Step 2: result -->
      <div v-else class="flex flex-col gap-4">
        <ul class="flex flex-col gap-2">
          <li
            v-for="result in results"
            :key="result.title"
            class="rounded-lg border p-3 text-sm dark:border-gray-800"
            :class="result.success ? 'text-green-500' : 'text-danger'"
          >
            <i :class="result.success ? 'fas fa-check' : 'fas fa-times'" class="mr-2"></i>
            <span class="font-semibold">{{ result.title }}</span>
            <span v-if="!result.success"> — {{ result.error }}</span>
          </li>
        </ul>
        <RouterLink
          to="/"
          class="tum-live-button-primary tum-live-input-submit px-4 py-3 text-center"
        >
          Return to homepage
        </RouterLink>
      </div>
    </section>
  </AdminLayout>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
