<script setup lang="ts">
import { ref } from "vue";

import type { WatchCameraPreset } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import { switchCameraPreset } from "@/lib/watch-admin";

/**
 * The camera positions of the lecture hall, as pictures: pressing one moves the camera
 * there. The server answers once the camera has had time to arrive, so the button stays
 * busy for a few seconds, and a second press meanwhile would only queue another move.
 */
const props = defineProps<{
  courseId: number;
  streamId: number;
  presets: readonly WatchCameraPreset[];
}>();

const busy = ref<number | null>(null);
/** The preset the camera was last sent to by this page. Not asked of the camera, which cannot say. */
const current = ref<number | null>(null);
const error = ref("");

async function choose(preset: WatchCameraPreset): Promise<void> {
  if (busy.value !== null) return;
  busy.value = preset.presetId;
  error.value = "";
  try {
    await switchCameraPreset(props.courseId, props.streamId, preset.presetId);
    current.value = preset.presetId;
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : "The camera could not be moved.";
  } finally {
    busy.value = null;
  }
}
</script>

<template>
  <section v-if="presets.length" aria-label="Camera presets" class="flex flex-col gap-2">
    <h3 class="text-3 text-sm font-semibold">Camera presets</h3>
    <ul class="flex flex-wrap gap-2">
      <li v-for="preset in presets" :key="preset.presetId">
        <button
          type="button"
          class="relative block w-28 overflow-hidden rounded border-2 text-left disabled:opacity-60"
          :class="current === preset.presetId ? 'border-blue-500' : 'border-transparent hover:border-gray-400'"
          :disabled="busy !== null"
          :aria-pressed="current === preset.presetId"
          :aria-busy="busy === preset.presetId"
          :data-testid="`preset-${preset.presetId}`"
          @click="choose(preset)"
        >
          <img :src="preset.imageUrl" :alt="`Preview of ${preset.name}`" class="aspect-video w-full bg-black object-cover" loading="lazy" />
          <span class="text-3 block truncate bg-gray-100 px-1 py-0.5 text-xs dark:bg-gray-800">
            {{ busy === preset.presetId ? "Moving…" : preset.name }}
          </span>
        </button>
      </li>
    </ul>
    <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-1 text-sm" role="alert">{{ error }}</p>
  </section>
</template>
