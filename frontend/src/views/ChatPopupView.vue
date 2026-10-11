<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";

import ChatPanel from "@/components/chat/ChatPanel.vue";
import NotFoundView from "@/views/NotFoundView.vue";

/**
 * A stream's chat on a page of its own, for a window beside the player. The slug in
 * the path is decoration; the stream id is what the API takes, and it answers for
 * whether this viewer may read the chat.
 */
const route = useRoute();
const streamId = computed(() => Number(route.params.streamID));
const valid = computed(() => Number.isInteger(streamId.value) && streamId.value > 0);
</script>

<template>
  <div class="h-full w-full">
    <ChatPanel v-if="valid" :key="streamId" :stream-id="streamId" />
    <NotFoundView v-else />
  </div>
</template>
