<script setup lang="ts">
import { applyUpdate, dismissUpdate, online, updateAvailable } from "@/lib/pwa";

/**
 * The service worker's two messages. The offline one explains why pages come up empty:
 * the shell loads from the cache, but nothing the API returns does. The update is
 * offered rather than applied, because applying it reloads the tab and would cut off a
 * lecture that is playing.
 */
</script>

<template>
  <div
    v-if="!online"
    role="status"
    data-testid="offline-notice"
    class="tum-live-notification tum-live-notification-warn fixed bottom-4 left-1/2 z-50 flex w-[calc(100%-2rem)] max-w-md -translate-x-1/2 items-center shadow-lg"
  >
    <i class="icon fa-solid fa-wifi"></i>
    <span class="title">You're offline. Content will load once you reconnect.</span>
  </div>
  <div
    v-else-if="updateAvailable"
    role="status"
    data-testid="update-notice"
    class="tum-live-notification tum-live-notification-info fixed bottom-4 left-1/2 z-50 flex w-[calc(100%-2rem)] max-w-md -translate-x-1/2 items-center shadow-lg"
  >
    <i class="icon fa-solid fa-rotate"></i>
    <span class="title">A new version of TUM-Live is available.</span>
    <button type="button" class="ml-auto px-2 font-semibold" @click="applyUpdate">Reload</button>
    <button type="button" class="px-2" aria-label="Dismiss" @click="dismissUpdate">
      <i class="fa-solid fa-xmark"></i>
    </button>
  </div>
</template>
