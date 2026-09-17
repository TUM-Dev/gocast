<script setup lang="ts">
import { onMounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import { AUDITS_PAGE_SIZE, fetchAudits, type AuditEntry } from "@/lib/audits";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * The server-wide audit log. Read-only, so there is nothing here beyond paging
 * through it -- see lib/audits.ts for why a page is fetched at a time rather than
 * the whole log.
 */
const auth = useAuthStore();

const pageSize = AUDITS_PAGE_SIZE;
const audits = ref<AuditEntry[]>([]);
const offset = ref(0);
const loading = ref(true);
const error = ref("");
/**
 * Whether a further page might exist. There is no total count, so this is a guess
 * from the page just fetched: a page shorter than the limit cannot be followed by
 * another.
 */
const hasMore = ref(true);

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to view the audit log.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function load(): Promise<void> {
  loading.value = true;
  try {
    const page = await fetchAudits(offset.value, pageSize);
    audits.value = page;
    hasMore.value = page.length === pageSize;
    error.value = "";
  } catch (err) {
    error.value = message(err);
  } finally {
    loading.value = false;
  }
}

function go(nextOffset: number): void {
  offset.value = Math.max(0, nextOffset);
  void load();
}

function formatDate(date: Date | null): string {
  return date ? date.toLocaleString() : "unknown";
}

onMounted(async () => {
  // Only reachable anonymously by a client-side navigation; a full load is refused.
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
    return;
  }

  await load();
});
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-6">
      <div class="flex items-center justify-between">
        <h1 class="text-1 text-2xl font-bold">Audits</h1>
      </div>

      <Transition name="fade" mode="out-in">
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
          {{ error }}
        </p>
      </Transition>

      <div class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800">
        <p v-if="loading" class="text-5 text-sm">Loading audits…</p>

        <!--
          An empty page is an ordinary state, not a failure: a fresh deployment has
          logged nothing yet, and paging past the end shows nothing further.
        -->
        <p v-else-if="!audits.length" class="text-5 text-sm">
          {{ offset === 0 ? "No audit entries yet." : "No further audit entries." }}
        </p>

        <ul v-else class="flex flex-col divide-y dark:divide-gray-800" role="list">
          <li v-for="audit in audits" :key="audit.id" class="py-4 first:pt-0 last:pb-0">
            <p class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 font-semibold">
              <span class="text-1">{{ audit.type }}</span>
              <span class="text-4 text-xs font-normal whitespace-nowrap">
                <span class="mr-4">{{ formatDate(audit.createdAt) }}</span>
                <span v-if="audit.userId">
                  <i class="fas fa-user"></i> {{ audit.userName }} ({{ audit.userId }})
                </span>
              </span>
            </p>
            <p class="text-3 text-sm">{{ audit.message }}</p>
          </li>
        </ul>

        <div class="flex justify-between border-t pt-4 dark:border-gray-800">
          <button
            type="button"
            class="text-5 font-semibold enabled:hover:text-1 disabled:opacity-50"
            :disabled="offset === 0"
            @click="go(offset - pageSize)"
          >
            <i class="fa-solid fa-angles-left mr-2"></i>Previous
          </button>
          <button
            type="button"
            class="text-5 font-semibold enabled:hover:text-1 disabled:opacity-50"
            :disabled="!hasMore"
            @click="go(offset + pageSize)"
          >
            Next<i class="fa-solid fa-angles-right ml-2"></i>
          </button>
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
