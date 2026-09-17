<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  REFRESH_INTERVAL_MS,
  deleteRunner,
  fetchRunners,
  timeAgo,
  type Runner,
} from "@/lib/runners";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/** The runners page. Refetches on an interval; see lib/runners.ts for why. */
const auth = useAuthStore();

const runners = ref<Runner[]>([]);
const loading = ref(true);
const error = ref("");
/** Redrawn on each tick so the "registered N minutes ago" column advances. */
const now = ref(new Date());

let timer: ReturnType<typeof setInterval> | undefined;

async function load(): Promise<void> {
  try {
    runners.value = await fetchRunners();
    now.value = new Date();
    error.value = "";
  } catch (err) {
    error.value = message(err);
  } finally {
    loading.value = false;
  }
}

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to administer runners.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

onMounted(async () => {
  // Only reachable anonymously by a client-side navigation; a full load is refused.
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
    return;
  }

  await load();
  timer = setInterval(load, REFRESH_INTERVAL_MS);
});

// Without this the poll outlives the page and keeps requesting after navigating away.
onUnmounted(() => {
  if (timer) clearInterval(timer);
});

async function remove(runner: Runner): Promise<void> {
  // The prompt has to say this clears an entry, not that it stops a machine.
  const confirmed = window.confirm(
    `Remove the registration for ${runner.hostname}? ` +
      `A runner that is still running will register again on its next heartbeat.`,
  );
  if (!confirmed) return;

  try {
    await deleteRunner(runner.hostname);
    await load();
  } catch (err) {
    error.value = message(err);
  }
}
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-4">
      <h1 class="text-1 text-2xl font-bold">Runners</h1>

      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
        {{ error }}
      </p>

      <p v-if="loading" class="text-5 text-sm">Loading runners…</p>

      <!--
        An empty list is an ordinary state, not a failure: a deployment can simply
        have no runners registered.
      -->
      <p v-else-if="!runners.length" class="text-5 text-sm">
        No runners are registered. They appear here once one registers itself.
      </p>

      <!-- Wide content scrolls inside its own container rather than the page. -->
      <div v-else class="overflow-x-auto">
        <table class="w-full table-auto text-left text-sm">
          <thead class="text-2 text-xs uppercase tracking-wide">
            <tr>
              <th scope="col" class="py-3 pr-6">Name</th>
              <th scope="col" class="px-6 py-3">Status</th>
              <th scope="col" class="px-6 py-3">Workload</th>
              <th scope="col" class="px-6 py-3">Registered</th>
              <th scope="col" class="px-6 py-3">Actions</th>
            </tr>
          </thead>
          <tbody class="text-3">
            <tr
              v-for="runner in runners"
              :key="runner.hostname"
              class="border-t dark:border-gray-800"
            >
              <td class="py-3 pr-6">
                <span class="text-1 font-semibold">{{ runner.hostname }}</span>
                <span class="text-4 font-normal"> @ {{ runner.version }}</span>
              </td>
              <td class="px-6 py-3">
                <span
                  class="rounded-full px-2 py-1 text-xs font-bold text-gray-100"
                  :class="runner.alive ? 'bg-green-500' : 'bg-red-500'"
                  >{{ runner.alive ? "Alive" : "Dead" }}</span
                >
                <!--
                  Draining is why a live runner stops picking up work, so it belongs
                  beside the status rather than hidden in the row detail the old page
                  never filled in.
                -->
                <span v-if="runner.draining" class="text-4 ml-2 text-xs">draining</span>
              </td>
              <td class="px-6 py-3 whitespace-nowrap">{{ runner.jobCount }} Jobs</td>
              <td class="px-6 py-3 whitespace-nowrap">
                {{ timeAgo(runner.registeredAt, now) }} ago
              </td>
              <td class="px-6 py-3">
                <button
                  type="button"
                  class="text-5 hover:text-1"
                  :title="`Remove ${runner.hostname}`"
                  :aria-label="`Remove ${runner.hostname}`"
                  @click="remove(runner)"
                >
                  <i class="fas fa-trash"></i>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </AdminLayout>
</template>
