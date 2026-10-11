<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";

import {
  decideIntegrationAuthorization,
  fetchIntegrationAuthorization,
  integrationAuthorizationErrorMessage,
  type IntegrationAuthorization,
} from "@/lib/integration-authorization";
import { singleQueryParam } from "@/lib/route-query";

/**
 * The consent page an application sends a lecturer to before it may read one of their
 * courses. Nothing happens until the user presses a button: approving or cancelling
 * asks the API for the application's return URL, then navigates there. Go serves this
 * page unframeable (see serveIntegrationAuthorization), so a framing page cannot
 * click through it.
 */
const route = useRoute();
const integrationId = computed(() => Number(route.params.id));
const state = computed(() => singleQueryParam(route.query, "state") ?? "");

const authorization = ref<IntegrationAuthorization | null>(null);
const courseId = ref(0);
const loadError = ref("");
const error = ref("");
// Stays set after success: the browser is on its way to the application, and a second
// press would only record another grant.
const working = ref(false);

onMounted(async () => {
  try {
    const loaded = await fetchIntegrationAuthorization(integrationId.value, state.value);
    authorization.value = loaded;
    courseId.value = loaded.courses[0]?.id ?? 0;
  } catch (err) {
    loadError.value = integrationAuthorizationErrorMessage(err);
  }
});

async function decide(decision: "approve" | "cancel"): Promise<void> {
  working.value = true;
  error.value = "";
  try {
    const target = await decideIntegrationAuthorization(
      integrationId.value,
      state.value,
      decision,
      courseId.value,
    );
    window.location.assign(target);
  } catch (err) {
    error.value = integrationAuthorizationErrorMessage(err);
    working.value = false;
  }
}
</script>

<template>
  <main class="text-3 mx-auto grid w-full max-w-2xl content-start gap-y-4 p-6">
    <p v-if="loadError" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ loadError }}</p>

    <p v-else-if="!authorization" class="text-5 text-sm">Loading…</p>

    <section v-else class="grid gap-y-4" aria-labelledby="approval-title">
      <h1 id="approval-title">
        <span class="text-1 block text-2xl font-bold [overflow-wrap:anywhere]">{{ authorization.integrationName }}</span>
        <span class="text-5 mt-1 block text-base font-medium">wants to connect to a course</span>
      </h1>
      <form class="grid min-w-0 gap-y-4" @submit.prevent="decide('approve')">
        <p class="text-5 min-w-0 text-sm">
          Signed in as <span class="text-3 font-medium [overflow-wrap:anywhere]">{{ authorization.userName }}</span>
        </p>
        <div v-if="authorization.courses.length" class="min-w-0">
          <label for="course_id" class="mb-2 block font-medium">Course</label>
          <select id="course_id" v-model="courseId" class="tum-live-input min-w-0 max-w-full" required>
            <option v-for="course in authorization.courses" :key="course.id" :value="course.id">
              {{ course.name }} ({{ course.teachingTerm }} {{ course.year }})
            </option>
          </select>
        </div>
        <p v-else class="rounded-lg border border-gray-200 p-4 dark:border-gray-700">
          You do not administer any courses that can be connected.
        </p>
        <div class="min-w-0">
          <h2 class="font-medium">This application can</h2>
          <ul class="mt-2 grid gap-2">
            <li class="flex items-start"><span class="text-success mr-3" aria-hidden="true">✓</span><span>Identify the selected course</span></li>
            <li class="flex items-start"><span class="text-success mr-3" aria-hidden="true">✓</span><span>Read its name, slug, and visibility</span></li>
          </ul>
          <p class="text-5 mt-3 text-sm">It cannot access course settings or private videos.</p>
        </div>
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>
        <div class="flex flex-wrap gap-3">
          <button
            v-if="authorization.courses.length"
            type="submit"
            class="tum-live-button tum-live-button-primary px-4 py-2"
            :disabled="working"
          >
            Authorize access
          </button>
          <button type="button" class="tum-live-button tum-live-button-secondary px-4 py-2" :disabled="working" @click="decide('cancel')">
            Cancel
          </button>
        </div>
      </form>
    </section>
  </main>
</template>
