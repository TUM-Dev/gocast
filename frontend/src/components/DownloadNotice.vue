<script setup lang="ts">
import { dismissDownloadNotice, downloadNoticeVisible } from "@/lib/download-notice";

/**
 * Tells the viewer that a recording download is being prepared. The edge builds the file
 * before sending it, so after the click the browser shows nothing for a while and the
 * click reads as broken. Mounted once by the app shell and fixed to the screen, so it
 * does not depend on the menu the link was in staying open.
 */
</script>

<template>
  <div
    v-if="downloadNoticeVisible"
    role="status"
    data-testid="download-notice"
    class="tum-live-notification tum-live-notification-info fixed bottom-4 left-1/2 z-50 flex w-[calc(100%-2rem)] max-w-md -translate-x-1/2 items-center shadow-lg"
  >
    <i class="icon fa-solid fa-spinner fa-spin"></i>
    <span class="title">Preparing your download, it will start in a few seconds.</span>
    <button
      type="button"
      class="ml-auto px-2"
      aria-label="Dismiss"
      @click="dismissDownloadNotice"
    >
      <i class="fa-solid fa-xmark"></i>
    </button>
  </div>
</template>
