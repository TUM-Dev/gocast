<script setup lang="ts">
import { computed, onMounted, ref } from "vue";

import { fetchNotifications, markAllRead, type Notification } from "@/lib/notifications";
import { useClickOutside } from "@/lib/useClickOutside";

const notifications = ref<Notification[]>([]);
const open = ref(false);
const root = ref<HTMLElement | null>(null);

const hasNew = computed(() => notifications.value.some((n) => !n.read));

useClickOutside(root, () => {
  open.value = false;
});

onMounted(async () => {
  try {
    notifications.value = await fetchNotifications();
  } catch {
    // A failed notification fetch must never take the page down with it.
    notifications.value = [];
  }
});

function toggle(): void {
  open.value = !open.value;
  if (open.value) {
    markAllRead(notifications.value);
  }
}
</script>

<template>
  <div ref="root" class="relative" @keyup.escape="open = false">
    <button
      type="button"
      title="Show Notifications"
      class="tum-live-icon-button p-3"
      @click="toggle"
    >
      <span
        v-show="hasNew"
        class="absolute right-1 top-1 h-3 w-3 rounded-full border-2 border-white bg-gradient-to-r from-cyan-500 to-blue-500 dark:border-secondary"
      ></span>
      <i class="fa-solid fa-bell"></i>
    </button>

    <div
      v-show="open"
      class="fixed bottom-0 left-0 top-0 w-full origin-top-right px-2 py-8 backdrop-brightness-50 md:absolute md:bottom-auto md:left-auto md:right-0 md:top-auto md:mt-2 md:w-96 md:p-0 md:backdrop-brightness-100"
    >
      <div class="tum-live-menu">
        <header>
          Notifications
          <button
            type="button"
            class="tum-live-icon-button close p-1 md:hidden"
            title="Close"
            @click="open = false"
          >
            <i class="fa-solid fa-xmark"></i>
          </button>
        </header>
        <div id="notification-list" class="max-h-60 min-h-30 w-full overflow-y-scroll py-2">
          <div v-if="notifications.length === 0" class="text-3 relative py-4 text-center">
            <span class="font-semibold">No notifications yet :)</span>
          </div>
          <div class="grid">
            <div
              v-for="notification in notifications"
              :key="notification.key"
              class="relative border-b px-4 py-3 last:border-0 dark:border-gray-800"
            >
              <p v-if="notification.title" class="mb-2 font-semibold">{{ notification.title }}</p>
              <!-- eslint-disable-next-line vue/no-v-html -->
              <!-- Bodies are sanitised server-side before they are stored (see api/notifications.go). -->
              <div class="notificationBody" v-html="notification.body"></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
