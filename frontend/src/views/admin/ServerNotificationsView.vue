<script setup lang="ts">
import { onMounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  createServerNotification,
  deleteServerNotification,
  fetchServerNotificationsAdmin,
  updateServerNotification,
  type AdminServerNotification,
} from "@/lib/server-notifications";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Server notification management: the site-wide banners shown between a start and
 * expiry time, e.g. announcing scheduled maintenance. Distinct from the per-user
 * notifications on /admin/notifications, which target a role rather than everyone.
 */
const auth = useAuthStore();

const notifications = ref<AdminServerNotification[]>([]);
const loading = ref(true);
const error = ref("");
const status = ref("");

/** input[type=datetime-local] wants "YYYY-MM-DDTHH:mm" in local time, not ISO/UTC. */
function toLocalInput(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** The row currently open for editing, or null when none is. */
const editingId = ref<number | null>(null);
const editText = ref("");
const editWarn = ref(false);
const editStart = ref("");
const editExpires = ref("");
const saving = ref(false);

const newText = ref("");
const newWarn = ref(false);
const newStart = ref("");
const newExpires = ref("");
const creating = ref(false);
const showCreateForm = ref(false);

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to administer server notifications.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function load(): Promise<void> {
  try {
    notifications.value = await fetchServerNotificationsAdmin();
    error.value = "";
  } catch (err) {
    error.value = message(err);
  } finally {
    loading.value = false;
  }
}

onMounted(async () => {
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
    return;
  }
  await load();
});

function edit(notification: AdminServerNotification): void {
  editingId.value = notification.id;
  editText.value = notification.text;
  editWarn.value = notification.warn;
  editStart.value = toLocalInput(notification.start);
  editExpires.value = toLocalInput(notification.expires);
  status.value = "";
}

function cancelEdit(): void {
  editingId.value = null;
}

async function saveEdit(notification: AdminServerNotification): Promise<void> {
  error.value = "";
  status.value = "";
  saving.value = true;
  try {
    const updated = await updateServerNotification(
      notification.id,
      editText.value.trim(),
      editWarn.value,
      new Date(editStart.value),
      new Date(editExpires.value),
    );
    const index = notifications.value.findIndex((n) => n.id === notification.id);
    if (index !== -1) notifications.value[index] = updated;
    editingId.value = null;
    status.value = "Saved.";
  } catch (err) {
    error.value = message(err);
  } finally {
    saving.value = false;
  }
}

async function remove(notification: AdminServerNotification): Promise<void> {
  if (!window.confirm(`Delete "${notification.text}"? It stops showing immediately.`)) return;

  error.value = "";
  status.value = "";
  try {
    await deleteServerNotification(notification.id);
    notifications.value = notifications.value.filter((n) => n.id !== notification.id);
    status.value = "Deleted.";
  } catch (err) {
    error.value = message(err);
  }
}

async function create(): Promise<void> {
  error.value = "";
  status.value = "";
  creating.value = true;
  try {
    const created = await createServerNotification(
      newText.value.trim(),
      newWarn.value,
      new Date(newStart.value),
      new Date(newExpires.value),
    );
    notifications.value.push(created);
    status.value = "Created.";
    newText.value = "";
    newWarn.value = false;
    newStart.value = "";
    newExpires.value = "";
    showCreateForm.value = false;
  } catch (err) {
    error.value = message(err);
  } finally {
    creating.value = false;
  }
}
</script>

<template>
  <AdminLayout>
    <section class="flex w-full flex-col gap-6">
      <div class="flex items-center justify-between">
        <h1 class="text-1 text-2xl font-bold">Server Notifications</h1>
        <button
          type="button"
          class="tum-live-button-primary px-4 py-2 text-sm"
          @click="showCreateForm = !showCreateForm"
        >
          {{ showCreateForm ? "Cancel" : "Add notification" }}
        </button>
      </div>

      <form
        v-if="showCreateForm"
        class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800"
        @submit.prevent="create"
      >
        <h2 class="text-1 font-semibold">New notification</h2>
        <div class="flex flex-col gap-1 text-sm">
          <label class="text-2" for="new-notification-text">Message</label>
          <input
            id="new-notification-text"
            v-model="newText"
            class="tum-live-input"
            placeholder="Enter message"
            autocomplete="off"
          />
        </div>
        <div class="flex flex-wrap gap-3">
          <div class="flex flex-1 flex-col gap-1 text-sm">
            <label class="text-2" for="new-notification-start">From</label>
            <input
              id="new-notification-start"
              v-model="newStart"
              type="datetime-local"
              class="tum-live-input"
            />
          </div>
          <div class="flex flex-1 flex-col gap-1 text-sm">
            <label class="text-2" for="new-notification-expires">Expires</label>
            <input
              id="new-notification-expires"
              v-model="newExpires"
              type="datetime-local"
              class="tum-live-input"
            />
          </div>
        </div>
        <fieldset class="flex flex-col gap-1 text-sm">
          <legend class="text-2">Type</legend>
          <div class="flex gap-4">
            <label class="flex items-center gap-2 text-3">
              <input v-model="newWarn" type="radio" :value="false" name="new-notification-type" />
              Info
            </label>
            <label class="flex items-center gap-2 text-3">
              <input v-model="newWarn" type="radio" :value="true" name="new-notification-type" />
              Warning
            </label>
          </div>
        </fieldset>
        <button
          type="submit"
          class="tum-live-input-submit tum-live-button-primary px-4 py-2 text-sm"
          :disabled="creating || !newText.trim() || !newStart || !newExpires"
        >
          {{ creating ? "Submitting…" : "Submit notification" }}
        </button>
      </form>

      <Transition name="fade" mode="out-in">
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
          {{ error }}
        </p>
        <p v-else-if="status" class="text-5 text-sm" role="status">{{ status }}</p>
      </Transition>

      <div class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800">
        <p v-if="loading" class="text-5 text-sm">Loading notifications…</p>
        <p v-else-if="!notifications.length" class="text-5 text-sm">No notifications yet.</p>

        <ul v-else class="flex flex-col gap-3">
          <li
            v-for="notification in notifications"
            :key="notification.id"
            class="rounded-lg border p-4 dark:border-gray-800"
          >
            <div
              v-if="editingId !== notification.id"
              class="flex items-center justify-between gap-4"
            >
              <div>
                <p class="text-1 font-semibold">
                  {{ notification.text }}
                  <span
                    class="ml-2 rounded-full px-2 py-1 text-xs font-bold text-gray-100"
                    :class="notification.warn ? 'bg-warn' : 'bg-gray-500'"
                  >
                    {{ notification.warn ? "Warning" : "Info" }}
                  </span>
                </p>
                <p class="text-5 text-sm">
                  {{ notification.start.toLocaleString() }} – {{ notification.expires.toLocaleString() }}
                </p>
              </div>
              <div class="flex items-center gap-4">
                <button
                  type="button"
                  class="text-5 hover:text-1"
                  title="Edit notification"
                  :aria-label="`Edit ${notification.text}`"
                  @click="edit(notification)"
                >
                  <i class="fas fa-pen"></i>
                </button>
                <button
                  type="button"
                  class="text-5 hover:text-1"
                  title="Delete notification"
                  :aria-label="`Delete ${notification.text}`"
                  @click="remove(notification)"
                >
                  <i class="fas fa-trash"></i>
                </button>
              </div>
            </div>

            <form v-else class="flex flex-col gap-3" @submit.prevent="saveEdit(notification)">
              <div class="flex flex-col gap-1 text-sm">
                <label class="text-2" :for="`edit-text-${notification.id}`">Message</label>
                <input
                  :id="`edit-text-${notification.id}`"
                  v-model="editText"
                  class="tum-live-input"
                />
              </div>
              <div class="flex flex-wrap gap-3">
                <div class="flex flex-1 flex-col gap-1 text-sm">
                  <label class="text-2" :for="`edit-start-${notification.id}`">From</label>
                  <input
                    :id="`edit-start-${notification.id}`"
                    v-model="editStart"
                    type="datetime-local"
                    class="tum-live-input"
                  />
                </div>
                <div class="flex flex-1 flex-col gap-1 text-sm">
                  <label class="text-2" :for="`edit-expires-${notification.id}`">Expires</label>
                  <input
                    :id="`edit-expires-${notification.id}`"
                    v-model="editExpires"
                    type="datetime-local"
                    class="tum-live-input"
                  />
                </div>
              </div>
              <fieldset class="flex flex-col gap-1 text-sm">
                <legend class="text-2">Type</legend>
                <div class="flex gap-4">
                  <label class="flex items-center gap-2 text-3">
                    <input
                      v-model="editWarn"
                      type="radio"
                      :value="false"
                      :name="`edit-notification-type-${notification.id}`"
                    />
                    Info
                  </label>
                  <label class="flex items-center gap-2 text-3">
                    <input
                      v-model="editWarn"
                      type="radio"
                      :value="true"
                      :name="`edit-notification-type-${notification.id}`"
                    />
                    Warning
                  </label>
                </div>
              </fieldset>
              <div class="flex gap-3">
                <button
                  type="submit"
                  class="tum-live-input-submit tum-live-button-primary px-4 py-2 text-sm"
                  :disabled="saving || !editText.trim() || !editStart || !editExpires"
                >
                  {{ saving ? "Saving…" : "Save" }}
                </button>
                <button
                  type="button"
                  class="text-5 px-4 py-2 text-sm"
                  :disabled="saving"
                  @click="cancelEdit"
                >
                  Cancel
                </button>
              </div>
            </form>
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
