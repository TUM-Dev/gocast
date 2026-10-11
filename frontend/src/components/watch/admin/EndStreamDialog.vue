<script setup lang="ts">
import { nextTick, onMounted, ref } from "vue";

import { ApiError } from "@/lib/api";
import { endStream } from "@/lib/watch-admin";

/**
 * "Keep the recording after ending the stream?" Yes ends the stream and keeps what was
 * recorded; No ends it and throws the recording away; Cancel closes.
 */
const props = defineProps<{ courseId: number; streamId: number }>();
const emit = defineEmits<{ close: []; ended: [discarded: boolean] }>();

const busy = ref(false);
const error = ref("");
const keep = ref<HTMLButtonElement | null>(null);

onMounted(async () => {
  await nextTick();
  keep.value?.focus();
});

async function end(discard: boolean): Promise<void> {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    await endStream(props.courseId, props.streamId, discard);
    emit("ended", discard);
    emit("close");
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : "The stream could not be ended.";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
    @click.self="!busy && emit('close')"
    @keydown.esc="!busy && emit('close')"
  >
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="end-stream-title"
      class="text-3 w-full max-w-md rounded bg-gray-200 px-6 py-4 shadow-lg dark:bg-secondary-light"
    >
      <h2 id="end-stream-title" class="text-2 pb-3 font-bold">Keep the recording after ending the stream?</h2>
      <p v-if="error" class="mb-3 rounded-lg bg-danger/25 px-2 py-1 text-sm" role="alert">{{ error }}</p>
      <div class="flex flex-wrap gap-2 pt-2">
        <button ref="keep" type="button" class="w-24 rounded bg-green-600 p-1 font-bold text-white hover:bg-green-700 disabled:opacity-60" :disabled="busy" @click="end(false)">
          Yes
        </button>
        <button type="button" class="w-24 rounded bg-red-600 p-1 font-bold text-white hover:bg-red-700 disabled:opacity-60" :disabled="busy" @click="end(true)">
          No
        </button>
        <button type="button" class="w-24 rounded bg-gray-500 p-1 font-bold text-white hover:bg-gray-600 disabled:opacity-60" :disabled="busy" @click="emit('close')">
          Cancel
        </button>
      </div>
    </div>
  </div>
</template>
