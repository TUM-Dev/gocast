<script setup lang="ts">
import { ref, watch } from "vue";

import {
  fetchCourseIntegrationGrants,
  revokeCourseIntegrationGrant,
  type CourseIntegrationGrant,
} from "@/lib/course-admin";
import { courseError } from "@/stores/course-admin";

const props = defineProps<{ courseId: number }>();
const grants = ref<CourseIntegrationGrant[]>([]);
const loading = ref(true);
const busy = ref(false);
const error = ref("");

async function load(): Promise<void> {
  loading.value = true;
  try {
    grants.value = await fetchCourseIntegrationGrants(props.courseId);
    error.value = "";
  } catch (err) {
    error.value = courseError(err);
  } finally {
    loading.value = false;
  }
}

watch(() => props.courseId, load, { immediate: true });

async function revoke(grant: CourseIntegrationGrant): Promise<void> {
  if (!window.confirm(`Revoke ${grant.name}'s access to this course?`)) return;
  busy.value = true;
  error.value = "";
  try {
    await revokeCourseIntegrationGrant(props.courseId, grant.id);
    await load();
  } catch (err) {
    error.value = courseError(err);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <section
    class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800"
    aria-labelledby="cs-integrations"
  >
    <header>
      <h2 id="cs-integrations" class="text-1 text-lg font-semibold">
        Authorized applications
      </h2>
      <p class="text-5 text-sm">
        Applications authorized to read this course's name, slug, and
        visibility.
      </p>
    </header>
    <p
      v-if="error"
      class="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
      role="alert"
    >
      {{ error }}
    </p>
    <p v-if="loading && !grants.length" class="text-5 text-sm">
      Loading applications…
    </p>
    <table v-else class="w-full text-sm" aria-label="Authorized applications">
      <thead>
        <tr class="text-4 text-left">
          <th class="py-2 pr-2 font-semibold">Name</th>
          <th class="py-2 text-right font-semibold">
            <span class="sr-only">Actions</span>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="grant in grants"
          :key="grant.id"
          class="border-t dark:border-gray-800"
        >
          <td class="text-1 py-2 pr-2 break-all">{{ grant.name }}</td>
          <td class="py-2 text-right">
            <button
              type="button"
              class="rounded px-2 py-1 text-red-600 hover:text-red-700 disabled:opacity-50 dark:text-red-400 dark:hover:text-red-300"
              :aria-label="`Revoke access for ${grant.name}`"
              :disabled="busy"
              @click="revoke(grant)"
            >
              Revoke access
            </button>
          </td>
        </tr>
        <tr v-if="!grants.length">
          <td colspan="2" class="text-5 py-2 text-center italic">
            No applications authorized yet
          </td>
        </tr>
      </tbody>
    </table>
  </section>
</template>
