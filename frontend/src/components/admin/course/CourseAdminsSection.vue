<script setup lang="ts">
import { computed, ref, watch } from "vue";

import {
  addCourseAdmin,
  fetchCourseAdmins,
  MIN_USER_QUERY,
  removeCourseAdmin,
  searchUsersForCourse,
  type CourseAdminUser,
} from "@/lib/course-admin";
import { courseError } from "@/stores/course-admin";

/**
 * The course's administrators: who can moderate the chat, plan lectures and manage
 * the course. Replaces web/ts/courseAdminManagement.ts.
 */
const props = defineProps<{ courseId: number }>();

const admins = ref<CourseAdminUser[]>([]);
const loading = ref(true);
const error = ref("");

async function load(): Promise<void> {
  loading.value = true;
  try {
    admins.value = await fetchCourseAdmins(props.courseId);
    error.value = "";
  } catch (err) {
    error.value = courseError(err);
  } finally {
    loading.value = false;
  }
}

watch(() => props.courseId, load, { immediate: true });

const query = ref("");
const results = ref<CourseAdminUser[]>([]);
const searched = ref(false);
const searchError = ref("");

// The old field searched on every key press; this waits for a pause instead.
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let latestSearch = 0;
watch(query, (q) => {
  clearTimeout(searchTimer);
  const trimmed = q.trim();
  searched.value = false;
  searchError.value = "";
  if (trimmed.length < MIN_USER_QUERY) {
    latestSearch++;
    results.value = [];
    return;
  }
  searchTimer = setTimeout(async () => {
    const mine = ++latestSearch;
    try {
      const found = await searchUsersForCourse(props.courseId, trimmed);
      if (mine !== latestSearch) return;
      results.value = found;
      searched.value = true;
    } catch (err) {
      if (mine !== latestSearch) return;
      results.value = [];
      searchError.value = courseError(err);
    }
  }, 300);
});

const adminIds = computed(() => new Set(admins.value.map((a) => a.id)));

const busy = ref(false);

async function add(user: CourseAdminUser): Promise<void> {
  busy.value = true;
  error.value = "";
  try {
    await addCourseAdmin(props.courseId, user.id);
    query.value = "";
    await load();
  } catch (err) {
    error.value = courseError(err);
  } finally {
    busy.value = false;
  }
}

async function remove(user: CourseAdminUser): Promise<void> {
  if (!window.confirm(`Remove ${user.name} as an administrator of this course?`)) return;
  busy.value = true;
  error.value = "";
  try {
    await removeCourseAdmin(props.courseId, user.id);
    await load();
  } catch (err) {
    // Removing the last administrator answers 400 with the reason, shown as is.
    error.value = courseError(err);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <section class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800" aria-labelledby="cs-admins">
    <header>
      <h2 id="cs-admins" class="text-1 text-lg font-semibold">Course Admins</h2>
      <p class="text-5 text-sm">
        Add or remove users who can help moderate the chat, plan lectures and manage the course.
      </p>
    </header>

    <p
      v-if="error"
      class="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
      role="alert"
    >
      {{ error }}
    </p>

    <p v-if="loading && !admins.length" class="text-5 text-sm">Loading administrators…</p>
    <table v-else class="w-full text-sm" aria-label="Course administrators">
      <thead>
        <tr class="text-4 text-left">
          <th class="py-2 pr-2 font-semibold">Name</th>
          <th class="py-2 pr-2 font-semibold">Login</th>
          <th class="py-2 text-right font-semibold"><span class="sr-only">Actions</span></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="admin in admins" :key="admin.id" class="border-t dark:border-gray-800">
          <td class="text-1 py-2 pr-2">{{ admin.name }}</td>
          <td class="text-3 py-2 pr-2 break-all">{{ admin.login }}</td>
          <td class="py-2 text-right">
            <button
              type="button"
              class="text-4 rounded px-2 py-1 hover:text-red-600 disabled:opacity-50 dark:hover:text-red-400"
              :aria-label="`Remove ${admin.name}`"
              :title="admins.length === 1 ? 'A course needs at least one administrator' : 'Remove'"
              :disabled="busy"
              @click="remove(admin)"
            >
              <i class="fa-solid fa-trash" aria-hidden="true"></i>
            </button>
          </td>
        </tr>
        <tr v-if="!admins.length">
          <td colspan="3" class="text-5 py-2 text-center italic">No administrators yet</td>
        </tr>
      </tbody>
    </table>

    <div class="flex flex-col gap-1 text-sm">
      <label for="cs-admin-search" class="text-2">Add an administrator</label>
      <input
        id="cs-admin-search"
        v-model="query"
        type="search"
        autocomplete="off"
        placeholder="Name, login or email, e.g. ga21tum"
        class="tum-live-input"
      />
      <p v-if="searchError" class="text-xs text-red-700 dark:text-red-400">{{ searchError }}</p>
      <p v-else-if="searched && !results.length" class="text-5 text-xs">
        Nobody found. Only users who have signed in at least once can be found by name.
      </p>
    </div>

    <ul v-if="results.length" class="flex flex-col" aria-label="Search results">
      <li
        v-for="user in results"
        :key="user.id"
        class="flex items-center justify-between gap-2 border-t py-2 text-sm dark:border-gray-800"
      >
        <span class="min-w-0">
          <span class="text-1">{{ user.name }}</span>
          <span class="text-5 ml-2 break-all">{{ user.login }}</span>
        </span>
        <span v-if="adminIds.has(user.id)" class="text-5 shrink-0 text-xs">Already an admin</span>
        <button
          v-else
          type="button"
          class="tum-live-button-primary shrink-0 px-3 py-1 text-xs"
          :disabled="busy"
          @click="add(user)"
        >
          Add {{ user.name }}
        </button>
      </li>
    </ul>
  </section>
</template>
