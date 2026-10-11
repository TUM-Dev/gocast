<script setup lang="ts">
import { ref } from "vue";

import { announceOwnReaction } from "@/lib/reaction-floats";
import { addReaction, fetchAllowedReactions } from "@/lib/watch-actions";
import ActionDialog from "./ActionDialog.vue";

/**
 * React to a live stream with an emoji. The action bar mounts it for signed-in viewers
 * of a live stream only (as the old page did). The emoji list loads on first open. The
 * server lets one reaction through per cooldown and answers 429 otherwise, which shows
 * as a hint here rather than an error.
 */
const props = defineProps<{ streamId: number }>();

const open = ref(false);
const reactions = ref<string[]>([]);
const loaded = ref(false);
const message = ref("");
const busy = ref(false);

async function show(): Promise<void> {
  open.value = true;
  message.value = "";
  if (loaded.value) return;
  try {
    reactions.value = await fetchAllowedReactions(props.streamId);
    loaded.value = true;
  } catch {
    message.value = "Reactions could not be loaded.";
  }
}

async function pick(reaction: string): Promise<void> {
  if (busy.value) return;
  busy.value = true;
  try {
    const result = await addReaction(props.streamId, reaction);
    if (result === "cooldown") {
      message.value = "Please wait before reacting again.";
      return;
    }
    // Shown over the player at once; the server's copy of it is skipped there.
    announceOwnReaction(props.streamId, reaction);
    open.value = false;
  } catch {
    message.value = "The reaction could not be sent.";
  } finally {
    busy.value = false;
  }
}

</script>

<template>
  <button
    type="button"
    class="text-4 hover:text-1 flex h-8 w-8 items-center justify-center rounded"
    title="React to stream"
    aria-label="React to stream"
    @click="show"
  >
    <i class="fa-solid fa-face-smile text-lg"></i>
  </button>
  <ActionDialog :open="open" title="React to stream" @close="open = false">
    <div class="flex flex-wrap justify-center gap-2" role="group" aria-label="Reactions">
      <button
        v-for="r in reactions"
        :key="r"
        type="button"
        class="rounded p-2 text-3xl hover:bg-gray-100 dark:hover:bg-gray-600"
        :disabled="busy"
        @click="pick(r)"
      >
        {{ r }}
      </button>
    </div>
    <p v-if="message" role="status" class="text-5 mt-2 text-center text-sm">{{ message }}</p>
  </ActionDialog>
</template>
