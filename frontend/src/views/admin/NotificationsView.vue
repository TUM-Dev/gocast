<script setup lang="ts">
import { onMounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  createNotification,
  deleteNotification,
  fetchNotificationsAdmin,
  notificationTargetLabels,
  NotificationTarget,
  type Notification,
} from "@/lib/notifications-admin";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Broadcasts a notification to a group of users -- the bell shown in the header of
 * every page, not the server-notifications banner (a separate page, separate model).
 */
const auth = useAuthStore();

const notifications = ref<Notification[]>([]);
const loading = ref(true);
const error = ref("");

const targets = Object.values(NotificationTarget).filter(
  (value): value is NotificationTarget => typeof value === "number",
);

const newTitle = ref("");
const newBody = ref("");
const newTarget = ref<NotificationTarget>(NotificationTarget.All);
const creating = ref(false);

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to administer notifications.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function load(): Promise<void> {
  try {
    notifications.value = await fetchNotificationsAdmin();
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

async function create(): Promise<void> {
  error.value = "";
  creating.value = true;
  try {
    const created = await createNotification(newBody.value, newTarget.value, newTitle.value.trim());
    notifications.value.unshift(created);
    newTitle.value = "";
    newBody.value = "";
    newTarget.value = NotificationTarget.All;
  } catch (err) {
    error.value = message(err);
  } finally {
    creating.value = false;
  }
}

async function remove(notification: Notification): Promise<void> {
  const confirmed = window.confirm(
    `Delete ${notification.title ? `"${notification.title}"` : "this notification"}? ` +
      "It stops showing to users immediately.",
  );
  if (!confirmed) return;

  error.value = "";
  try {
    await deleteNotification(notification.id);
    notifications.value = notifications.value.filter((n) => n.id !== notification.id);
  } catch (err) {
    error.value = message(err);
  }
}
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-3xl flex-col gap-6">
      <h1 class="text-1 text-2xl font-bold">User Notifications</h1>

      <Transition name="fade" mode="out-in">
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
          {{ error }}
        </p>
      </Transition>

      <form
        class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800"
        @submit.prevent="create"
      >
        <h2 class="text-1 font-semibold">Create Notification</h2>
        <div class="flex flex-col gap-1 text-sm">
          <label class="text-2" for="notification-target">Notification Target</label>
          <select id="notification-target" v-model.number="newTarget" class="tum-live-input">
            <option v-for="target in targets" :key="target" :value="target">
              {{ notificationTargetLabels[target] }}
            </option>
          </select>
        </div>
        <div class="flex flex-col gap-1 text-sm">
          <label class="text-2" for="notification-title">Title (optional)</label>
          <input
            id="notification-title"
            v-model="newTitle"
            class="tum-live-input"
            placeholder="Enter Title"
            autocomplete="off"
          />
        </div>
        <div class="flex flex-col gap-1 text-sm">
          <label class="text-2" for="notification-body">Body (you can use Markdown)</label>
          <textarea
            id="notification-body"
            v-model="newBody"
            class="tum-live-input h-32 resize-none"
            placeholder="Enter Body"
            required
          ></textarea>
        </div>
        <button
          type="submit"
          class="tum-live-input-submit tum-live-button-primary px-4 py-2 text-sm"
          :disabled="creating || !newBody.trim()"
        >
          {{ creating ? "Creating…" : "Create" }}
        </button>
      </form>

      <div class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800">
        <p v-if="loading" class="text-5 text-sm">Loading notifications…</p>
        <p v-else-if="!notifications.length" class="text-5 text-sm">No notifications yet.</p>

        <ul v-else class="flex flex-col gap-3">
          <li
            v-for="notification in notifications"
            :key="notification.id"
            class="rounded-lg border p-4 dark:border-gray-800"
          >
            <div class="flex items-center justify-between gap-4">
              <div>
                <div class="text-5 text-sm">
                  {{ notification.createdAt?.toLocaleString() ?? "unknown time" }}
                </div>
                <i class="text-1">{{ notification.title ?? "No title" }}</i>
              </div>
              <button
                type="button"
                class="text-5 hover:text-1"
                :title="`Delete ${notification.title ?? 'this notification'}`"
                :aria-label="`Delete ${notification.title ?? 'this notification'}`"
                @click="remove(notification)"
              >
                <i class="fas fa-trash"></i>
              </button>
            </div>
            <!-- v-html is safe only because bluemonday already ran server-side; do not
                 feed unsanitised content through this. -->
            <div class="tum-live-markdown text-3 mt-2 text-sm" v-html="notification.body"></div>
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
