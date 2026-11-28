<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  deleteEmailFailure,
  deleteTranscodingFailure,
  dryRunReap,
  executeReap,
  fetchAllReapEntries,
  fetchCronJobs,
  fetchEmailFailures,
  fetchTranscodingFailures,
  runCronJob,
  type EmailFailure,
  type ReapEntry,
  type ReapPlanPage,
  type ReapResult,
  type TranscodingFailure,
} from "@/lib/maintenance";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Server maintenance: cron jobs, and dismissing failed transcodings and emails. Four
 * independent panels, matching the old page — nothing here shares state, so one
 * panel's error does not block the others from loading.
 */
const auth = useAuthStore();
const error = ref("");

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to administer the server.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

// --- Cron jobs ---------------------------------------------------------------------

const cronJobs = ref<string[]>([]);
const selectedCronJob = ref("");
const cronRunOk = ref<boolean | null>(null);
const runningCronJob = ref(false);
let cronStatusTimer: ReturnType<typeof setTimeout> | undefined;

async function loadCronJobs(): Promise<void> {
  try {
    cronJobs.value = await fetchCronJobs();
  } catch (err) {
    error.value = message(err);
  }
}

async function runSelectedCronJob(): Promise<void> {
  if (!selectedCronJob.value || selectedCronJob.value === "---") return;

  // Triggers a job the old page also ran with a single click and no confirmation:
  // every job here already runs unattended on its own schedule, so running it once
  // more by hand is not a new risk the page needs to guard against.
  runningCronJob.value = true;
  try {
    await runCronJob(selectedCronJob.value);
    cronRunOk.value = true;
    selectedCronJob.value = "---";
  } catch (err) {
    cronRunOk.value = false;
    error.value = message(err);
  } finally {
    runningCronJob.value = false;
    if (cronStatusTimer) clearTimeout(cronStatusTimer);
    cronStatusTimer = setTimeout(() => {
      cronRunOk.value = null;
    }, 5000);
  }
}

// --- Failed transcodings -----------------------------------------------------------

const transcodingFailures = ref<TranscodingFailure[]>([]);
const expandedTranscodingFailures = ref(new Set<number>());
const loadingTranscodingFailures = ref(true);

async function loadTranscodingFailures(): Promise<void> {
  try {
    transcodingFailures.value = await fetchTranscodingFailures();
  } catch (err) {
    error.value = message(err);
  } finally {
    loadingTranscodingFailures.value = false;
  }
}

function toggleTranscodingFailure(id: number): void {
  if (expandedTranscodingFailures.value.has(id)) {
    expandedTranscodingFailures.value.delete(id);
  } else {
    expandedTranscodingFailures.value.add(id);
  }
}

async function removeTranscodingFailure(failure: TranscodingFailure): Promise<void> {
  if (
    !window.confirm(
      `Dismiss the transcoding failure for stream ${failure.streamId} (${failure.version})? ` +
        `This does not retry the transcode.`,
    )
  ) {
    return;
  }

  try {
    await deleteTranscodingFailure(failure.id);
    transcodingFailures.value = transcodingFailures.value.filter((f) => f.id !== failure.id);
  } catch (err) {
    error.value = message(err);
  }
}

// --- Failed emails -------------------------------------------------------------------

const emailFailures = ref<EmailFailure[]>([]);
const expandedEmailFailures = ref(new Set<number>());
const loadingEmailFailures = ref(true);

async function loadEmailFailures(): Promise<void> {
  try {
    emailFailures.value = await fetchEmailFailures();
  } catch (err) {
    error.value = message(err);
  } finally {
    loadingEmailFailures.value = false;
  }
}

function toggleEmailFailure(id: number): void {
  if (expandedEmailFailures.value.has(id)) {
    expandedEmailFailures.value.delete(id);
  } else {
    expandedEmailFailures.value.add(id);
  }
}

async function removeEmailFailure(failure: EmailFailure): Promise<void> {
  if (!window.confirm(`Dismiss the failed email to ${failure.to}? It will not be retried.`)) {
    return;
  }

  try {
    await deleteEmailFailure(failure.id);
    emailFailures.value = emailFailures.value.filter((f) => f.id !== failure.id);
  } catch (err) {
    error.value = message(err);
  }
}

// --- Deleted media -----------------------------------------------------------------

// Soft-deleting a stream leaves its files on disk. The dry run lists every file and
// directory a delete would remove; deleting needs that dry run's token, and the server
// refuses if anything changed since, so only what is shown here is ever removed.
const reapPlan = ref<ReapPlanPage | null>(null);
const reapEntries = ref<ReapEntry[]>([]);
const reapResult = ref<ReapResult | null>(null);
const reapBusy = ref(false);

function formatBytes(bytes: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

async function runReapDryRun(): Promise<void> {
  reapBusy.value = true;
  reapResult.value = null;
  try {
    const plan = await dryRunReap();
    reapPlan.value = plan;
    reapEntries.value = plan.entries;
  } catch (err) {
    error.value = message(err);
  } finally {
    reapBusy.value = false;
  }
}

async function loadMoreReapEntries(): Promise<void> {
  if (!reapPlan.value) return;
  reapBusy.value = true;
  try {
    const page = await dryRunReap(reapEntries.value.length, reapPlan.value.planToken);
    reapEntries.value = [...reapEntries.value, ...page.entries];
  } catch (err) {
    error.value = message(err);
  } finally {
    reapBusy.value = false;
  }
}

async function downloadReapList(): Promise<void> {
  if (!reapPlan.value) return;
  reapBusy.value = true;
  try {
    const all = await fetchAllReapEntries(reapPlan.value);
    const lines = all.map((e) => `${e.type}\t${e.bytes}\t${e.path}`);
    const url = URL.createObjectURL(new Blob([lines.join("\n") + "\n"], { type: "text/plain" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = "reap-deleted-media.txt";
    link.click();
    URL.revokeObjectURL(url);
  } catch (err) {
    error.value = message(err);
  } finally {
    reapBusy.value = false;
  }
}

async function runReap(): Promise<void> {
  const plan = reapPlan.value;
  if (!plan) return;
  if (
    !window.confirm(
      `Permanently delete ${plan.totalFiles} files (${formatBytes(plan.totalBytes)})? This cannot be undone.`,
    )
  ) {
    return;
  }

  reapBusy.value = true;
  try {
    reapResult.value = await executeReap(plan.planToken);
    reapPlan.value = null;
    reapEntries.value = [];
  } catch (err) {
    error.value = message(err);
    // The plan is stale after a refusal; make the admin review a fresh one.
    reapPlan.value = null;
    reapEntries.value = [];
  } finally {
    reapBusy.value = false;
  }
}

onMounted(async () => {
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
    return;
  }

  await Promise.all([loadCronJobs(), loadTranscodingFailures(), loadEmailFailures()]);
});

onUnmounted(() => {
  if (cronStatusTimer) clearTimeout(cronStatusTimer);
});
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-3xl flex-col gap-6">
      <h1 class="text-1 text-2xl font-bold">Maintenance</h1>

      <Transition name="fade" mode="out-in">
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
          {{ error }}
        </p>
      </Transition>

      <div class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800">
        <h2 class="text-1 font-semibold">Cron Jobs</h2>
        <div class="flex flex-col gap-2 md:flex-row">
          <select v-model="selectedCronJob" class="tum-live-input" aria-label="Cron job">
            <option>---</option>
            <option v-for="job in cronJobs" :key="job" :value="job">{{ job }}</option>
          </select>
          <button
            type="button"
            class="tum-live-button-primary px-4 py-2 text-sm"
            :disabled="runningCronJob || !selectedCronJob || selectedCronJob === '---'"
            @click="runSelectedCronJob"
          >
            Run
          </button>
        </div>
        <span
          v-if="cronRunOk !== null"
          class="text-sm"
          :class="cronRunOk ? 'text-green-500' : 'text-red-500'"
          role="status"
        >
          {{ cronRunOk ? "Job has been triggered." : "Something went wrong." }}
        </span>
      </div>

      <div class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800">
        <h2 class="text-1 font-semibold">Deleted Media</h2>
        <p class="text-5 text-sm">
          Deleting a stream or course leaves its video files on disk. A dry run lists every file and
          directory that would be removed, for streams deleted more than 30 days ago. Files that a live
          stream or a copy still uses are never listed.
        </p>
        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            class="tum-live-button-primary px-4 py-2 text-sm"
            :disabled="reapBusy"
            @click="runReapDryRun"
          >
            Dry run
          </button>
          <template v-if="reapPlan && reapPlan.totalEntries">
            <button type="button" class="px-4 py-2 text-sm" :disabled="reapBusy" @click="downloadReapList">
              Download full list
            </button>
            <button
              type="button"
              class="rounded bg-danger px-4 py-2 text-sm text-white"
              :disabled="reapBusy"
              @click="runReap"
            >
              Delete {{ reapPlan.totalFiles }} files ({{ formatBytes(reapPlan.totalBytes) }})
            </button>
          </template>
        </div>

        <p v-if="reapResult" class="text-sm" role="status">
          Deleted {{ reapResult.deletedFiles }} files ({{ formatBytes(reapResult.deletedBytes) }}).
          <span v-if="reapResult.errors.length" class="text-red-500">
            {{ reapResult.errors.length }} failed: {{ reapResult.errors.slice(0, 3).join("; ") }}
          </span>
        </p>

        <template v-if="reapPlan">
          <p v-if="!reapPlan.totalEntries" class="text-5 text-sm">Nothing to delete.</p>
          <template v-else>
            <p class="text-3 text-sm">
              {{ reapPlan.totalDirs }} directories, {{ reapPlan.totalFiles }} files,
              {{ formatBytes(reapPlan.totalBytes) }}
            </p>
            <ul class="flex max-h-96 flex-col gap-1 overflow-auto font-mono text-xs">
              <li v-for="entry in reapEntries" :key="entry.path" class="flex justify-between gap-4">
                <span class="text-3 break-all">{{ entry.type === "dir" ? entry.path + "/" : entry.path }}</span>
                <span class="text-5 shrink-0">
                  {{ entry.type === "dir" ? "" : formatBytes(entry.bytes) }} · stream
                  {{ entry.streamIds.join(", ") }}
                </span>
              </li>
            </ul>
            <button
              v-if="reapEntries.length < reapPlan.totalEntries"
              type="button"
              class="self-start text-sm"
              :disabled="reapBusy"
              @click="loadMoreReapEntries"
            >
              Show more ({{ reapPlan.totalEntries - reapEntries.length }} left)
            </button>
          </template>
          <details v-if="reapPlan.skipped.length" class="text-sm">
            <summary class="text-5 cursor-pointer">
              {{ reapPlan.skipped.length }} left alone
            </summary>
            <ul class="mt-2 flex max-h-64 flex-col gap-1 overflow-auto font-mono text-xs">
              <li v-for="skipped in reapPlan.skipped" :key="skipped.path + skipped.streamIds.join()">
                <span class="text-3 break-all">{{ skipped.path }}</span>
                <span class="text-5"> · {{ skipped.reason }}</span>
              </li>
            </ul>
          </details>
        </template>
      </div>

      <div class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800">
        <h2 class="text-1 font-semibold">Failed Transcodings</h2>
        <p v-if="loadingTranscodingFailures" class="text-5 text-sm">Loading…</p>
        <p v-else-if="!transcodingFailures.length" class="text-5 text-sm">No failed transcodings.</p>
        <ul v-else class="flex flex-col gap-2">
          <li
            v-for="failure in transcodingFailures"
            :key="failure.id"
            class="rounded border p-3 dark:border-gray-800"
          >
            <div class="flex items-center justify-between gap-4">
              <div class="text-3 text-sm">
                <span class="font-semibold">{{ failure.streamId }} - {{ failure.version }}</span>
                <span class="text-5"> · {{ failure.friendlyTime }} · {{ failure.hostname }}</span>
              </div>
              <div class="flex items-center gap-3">
                <button
                  type="button"
                  class="text-5 hover:text-1 text-sm"
                  @click="toggleTranscodingFailure(failure.id)"
                >
                  {{ expandedTranscodingFailures.has(failure.id) ? "Collapse" : "Expand" }}
                </button>
                <button
                  type="button"
                  class="text-5 hover:text-1"
                  :title="`Dismiss failure for stream ${failure.streamId}`"
                  :aria-label="`Dismiss failure for stream ${failure.streamId}`"
                  @click="removeTranscodingFailure(failure)"
                >
                  <i class="fas fa-trash"></i>
                </button>
              </div>
            </div>
            <div v-if="expandedTranscodingFailures.has(failure.id)" class="mt-2 text-sm">
              <p class="text-2 font-semibold">Filename</p>
              <p class="text-3">{{ failure.filePath }}</p>
              <p class="text-2 mt-2 font-semibold">Logs</p>
              <pre class="text-3 w-full overflow-x-auto whitespace-pre-wrap">{{ failure.logs }}</pre>
            </div>
          </li>
        </ul>
      </div>

      <div class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800">
        <h2 class="text-1 font-semibold">Failed Emails</h2>
        <p v-if="loadingEmailFailures" class="text-5 text-sm">Loading…</p>
        <p v-else-if="!emailFailures.length" class="text-5 text-sm">No failed emails.</p>
        <ul v-else class="flex flex-col gap-2">
          <li
            v-for="failure in emailFailures"
            :key="failure.id"
            class="rounded border p-3 dark:border-gray-800"
          >
            <div class="flex items-center justify-between gap-4">
              <div class="text-3 text-sm">
                <span class="font-semibold">{{ failure.to }}</span>
                <span class="text-5">
                  · {{ failure.lastTry ? failure.lastTry.toLocaleString() : "never tried" }} ·
                  {{ failure.retries }} attempts
                </span>
              </div>
              <div class="flex items-center gap-3">
                <button
                  type="button"
                  class="text-5 hover:text-1 text-sm"
                  @click="toggleEmailFailure(failure.id)"
                >
                  {{ expandedEmailFailures.has(failure.id) ? "Collapse" : "Expand" }}
                </button>
                <button
                  type="button"
                  class="text-5 hover:text-1"
                  :title="`Dismiss failed email to ${failure.to}`"
                  :aria-label="`Dismiss failed email to ${failure.to}`"
                  @click="removeEmailFailure(failure)"
                >
                  <i class="fas fa-trash"></i>
                </button>
              </div>
            </div>
            <div v-if="expandedEmailFailures.has(failure.id)" class="mt-2 text-sm">
              <p class="text-2 font-semibold">To/Subject</p>
              <p class="text-3">{{ failure.to }}: {{ failure.subject }}</p>
              <p class="text-2 mt-2 font-semibold">Body</p>
              <pre class="text-3 w-full overflow-x-auto whitespace-pre-wrap">{{ failure.body }}</pre>
              <p class="text-2 mt-2 font-semibold">Errors</p>
              <pre class="text-3 w-full overflow-x-auto whitespace-pre-wrap">{{ failure.errors }}</pre>
            </div>
          </li>
        </ul>
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
