<script setup lang="ts">
import { computed, onMounted } from "vue";
import { RouterLink } from "vue-router";

import { useOfflineStore } from "@/stores/offline";

/**
 * Saves a lecture for offline viewing, and shows how far that got. A menu item in the
 * watch page's downloads menu and the lecture cards' download list; the parent decides
 * whether to offer it (the installed app, and a lecture the viewer may download). The
 * item's look comes from the parent, through `class`.
 */
const props = defineProps<{ streamId: number }>();

const offline = useOfflineStore();
onMounted(() => offline.load());

const lecture = computed(() => offline.byStream(props.streamId));
const saving = computed(() => !!offline.saving[props.streamId]);
const error = computed(() => offline.errors[props.streamId]);
const label = computed(() => {
  const p = offline.progress[props.streamId];
  if (p !== undefined) return `Saving offline… ${Math.floor(p * 100)} %`;
  if (saving.value) return lecture.value?.state === "downloading" ? "Saving offline…" : "Preparing for offline…";
  if (lecture.value && lecture.value.state !== "saved") return "Resume saving offline";
  return "Save offline";
});
</script>

<template>
  <RouterLink v-if="lecture?.state === 'saved'" role="menuitem" to="/offline" data-testid="saved-offline">
    <i class="fa-solid fa-circle-check text-xs text-green-600 dark:text-green-400"></i>
    <span>Saved offline</span>
  </RouterLink>
  <button
    v-else
    type="button"
    role="menuitem"
    :disabled="saving"
    :aria-busy="saving"
    data-testid="save-offline"
    @click="offline.save(streamId)"
  >
    <i class="fa-solid text-xs text-gray-500 dark:text-gray-200" :class="saving ? 'fa-spinner fa-spin' : 'fa-cloud-arrow-down'"></i>
    <span class="flex flex-col items-start text-left">
      <span>{{ label }}</span>
      <span v-if="error && !saving" class="text-xs text-red-600 dark:text-red-400" role="alert">{{ error }}</span>
    </span>
  </button>
</template>
