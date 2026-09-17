<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import { REFRESH_INTERVAL_MS, deleteWorker, fetchWorkers, type Worker } from "@/lib/workers";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/** The workers page. Refetches on an interval; see lib/workers.ts for why. */
const auth = useAuthStore();

const workers = ref<Worker[]>([]);
const token = ref("");
const loading = ref(true);
const error = ref("");

/** Which "how to add a worker" tab is open. Mirrors the old page's Alpine state. */
const tab = ref<"plain" | "docker" | "swarm">("plain");

let timer: ReturnType<typeof setInterval> | undefined;

async function load(): Promise<void> {
  try {
    const page = await fetchWorkers();
    workers.value = page.workers;
    token.value = page.token;
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
    if (err.status === 403) return "You do not have permission to administer workers.";
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

async function remove(worker: Worker): Promise<void> {
  // The prompt has to say this clears an entry, not that it stops a machine.
  const confirmed = window.confirm(
    `Remove the registration for ${worker.host}? ` +
      `A worker that is still running will register again on its next heartbeat.`,
  );
  if (!confirmed) return;

  try {
    await deleteWorker(worker.workerId);
    await load();
  } catch (err) {
    error.value = message(err);
  }
}

const dockerCommand = computed(
  () => `docker run -p 50051:50051 -e "Host=vm1234" -e "Token=${token.value}" ghcr.io/TUM-Dev/gocast/worker:latest`,
);
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-6">
      <div class="flex items-center justify-between">
        <h1 class="text-1 text-2xl font-bold">Workers</h1>
      </div>

      <Transition name="fade" mode="out-in">
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
          {{ error }}
        </p>
      </Transition>

      <div class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800">
        <p v-if="loading" class="text-5 text-sm">Loading workers…</p>

        <!--
          An empty list is an ordinary state, not a failure: a deployment can simply
          have no workers registered.
        -->
        <p v-else-if="!workers.length" class="text-5 text-sm">
          No workers are registered. They appear here once one registers itself.
        </p>

        <!-- Wide content scrolls inside its own container rather than the page. -->
        <div v-else class="overflow-x-auto">
          <table class="w-full table-auto text-left text-sm">
            <thead class="text-2 text-xs uppercase tracking-wide">
              <tr>
                <th scope="col" class="py-3 pr-6">Name</th>
                <th scope="col" class="px-6 py-3">Status</th>
                <th scope="col" class="px-6 py-3">Workload</th>
                <th scope="col" class="px-6 py-3">Uptime</th>
                <th scope="col" class="px-6 py-3">Actions</th>
              </tr>
            </thead>
            <tbody class="text-3">
              <tr
                v-for="worker in workers"
                :key="worker.workerId"
                class="border-t dark:border-gray-800"
              >
                <td class="py-3 pr-6">
                  <div>
                    <span class="text-1 font-semibold">{{ worker.host }}</span>
                    <span class="text-4 font-normal"> @ {{ worker.version }}</span>
                  </div>
                  <div class="text-2 pl-1 text-xs italic">
                    <span class="mr-4">CPU: {{ worker.cpu }}</span>
                    <span class="mr-4">Mem: {{ worker.memory }}</span>
                    <span class="mr-4">Disk: {{ worker.disk }}</span>
                  </div>
                </td>
                <td class="px-6 py-3">
                  <span
                    class="rounded-full px-2 py-1 text-xs font-bold text-gray-100"
                    :class="worker.alive ? 'bg-green-500' : 'bg-red-500'"
                    >{{ worker.alive ? "Alive" : "Dead" }}</span
                  >
                  <span v-if="worker.status" class="text-4 ml-2 text-xs">{{ worker.status }}</span>
                </td>
                <td class="px-6 py-3 whitespace-nowrap">{{ worker.workload }}</td>
                <td class="px-6 py-3 whitespace-nowrap">{{ worker.uptime }}</td>
                <td class="px-6 py-3">
                  <button
                    type="button"
                    class="text-5 hover:text-1"
                    :title="`Remove ${worker.host}`"
                    :aria-label="`Remove ${worker.host}`"
                    @click="remove(worker)"
                  >
                    <i class="fas fa-trash"></i>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="flex flex-col gap-3">
        <h3 class="text-3 text-lg font-semibold">How to add a worker</h3>
        <div
          class="dark:border-secondary dark:bg-secondary-lighter overflow-x-auto rounded-md bg-gray-100 shadow-md"
        >
          <div
            class="dark:bg-secondary w-full rounded-t-md bg-gray-200 py-3 text-sm font-semibold uppercase leading-normal"
          >
            <button
              type="button"
              class="hover:text-1 cursor-pointer px-6"
              :class="tab === 'plain' ? 'text-1 font-bold' : 'text-4'"
              @click="tab = 'plain'"
            >
              Plain
            </button>
            <button
              type="button"
              class="hover:text-1 cursor-pointer border-x-2 border-gray-500 px-6"
              :class="tab === 'docker' ? 'text-1 font-bold' : 'text-4'"
              @click="tab = 'docker'"
            >
              Docker
            </button>
            <button
              type="button"
              class="hover:text-1 cursor-pointer px-6"
              :class="tab === 'swarm' ? 'text-1 font-bold' : 'text-4'"
              @click="tab = 'swarm'"
            >
              Docker Swarm
            </button>
          </div>

          <p v-if="tab === 'plain'" class="p-3">
            <span class="text-gray-500"
              ># Run the TUM-Live-Worker executable with these environment variables:</span
            ><br />
            <span class="block">export Token=<span class="text-cyan-500">{{ token }}</span></span>
            <span class="block">./worker</span>
          </p>

          <p
            v-else-if="tab === 'docker'"
            class="dark:bg-secondary-lighter dark:text-1 rounded bg-secondary p-3 text-white"
          >
            <span class="text-gray-500"
              ># Run the TUM-Live-Worker docker container with the token, make sure to include
              its hostname:</span
            ><br />
            <span class="block break-all">{{ dockerCommand }}</span>
          </p>

          <p
            v-else
            class="dark:bg-secondary-lighter dark:text-1 rounded bg-gray-400 p-3 text-white"
          >
            <span class="text-gray-500"
              ># Refer to your manager node on which token to use here:</span
            ><br />
            <span class="block">docker swarm join --token ABC-1243-DEFG 1.2.3.4:2377</span>
          </p>
        </div>
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
