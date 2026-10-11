<script setup lang="ts">
/**
 * Says that the viewer's watch progress could not be loaded. Without it the eye icons
 * and the "Hide watched" filter just silently vanish, which reads as "I have watched
 * nothing". Not blocking: the lectures themselves are all still there.
 */
defineProps<{ loading: boolean }>();
defineEmits<{ retry: [] }>();
</script>

<template>
  <div
    role="status"
    class="tum-live-notification tum-live-notification-warn mx-3 mb-3 flex items-center"
  >
    <i class="icon fa-solid fa-triangle-exclamation"></i>
    <span class="title">Couldn't load your watch progress</span>
    <button
      type="button"
      class="tum-live-button tum-live-button-tertiary ml-auto text-xs"
      :disabled="loading"
      @click="$emit('retry')"
    >
      Retry
    </button>
  </div>
</template>
