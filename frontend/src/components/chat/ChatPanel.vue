<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";

import ChatMessageItem from "@/components/chat/ChatMessageItem.vue";
import ChatPollCard from "@/components/chat/ChatPollCard.vue";
import { connectionNotice, countMessages } from "@/lib/chat-view";
import { useChatStore } from "@/stores/chat";

/**
 * A stream's chat, read-only: messages with their replies, the running poll, the
 * viewer count and the state of the connection. Everything comes from the chat store,
 * which the realtime socket keeps current; this only draws it.
 *
 * Sending is the next step. The disabled field at the bottom marks where it goes.
 */
const props = defineProps<{ streamId: number }>();

// A different stream is a different store, so the panel is remounted per stream (the
// view keys it) rather than switching stores in place.
const chat = useChatStore(props.streamId);

const notice = computed(() =>
  connectionNotice({
    status: chat.status,
    loaded: chat.loaded,
    error: chat.error,
    fatalCode: chat.fatalCode,
    lastCloseCode: chat.lastCloseCode,
  }),
);

const total = computed(() => countMessages(chat.messages));

/** Within this many pixels of the end counts as "at the bottom". */
const BOTTOM_SLACK = 48;

const list = ref<HTMLElement | null>(null);
const atBottom = ref(true);
const unseen = ref(0);

function scrollToEnd(): void {
  const el = list.value;
  if (!el) return;
  el.scrollTop = el.scrollHeight;
  atBottom.value = true;
  unseen.value = 0;
}

function onScroll(): void {
  const el = list.value;
  if (!el) return;
  atBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight <= BOTTOM_SLACK;
  if (atBottom.value) unseen.value = 0;
}

// Follow the newest message while the reader is at the end; once they have scrolled
// up to read, leave them there and offer the way back instead.
watch(total, async (now, before) => {
  if (atBottom.value) {
    await nextTick();
    scrollToEnd();
  } else if (now > before) {
    unseen.value += now - before;
  }
});

// The first snapshot lands after mount.
watch(
  () => chat.loaded,
  async () => {
    await nextTick();
    scrollToEnd();
  },
);

onMounted(() => chat.start());
onBeforeUnmount(() => chat.stop());
</script>

<template>
  <section class="flex h-full min-h-0 w-full flex-col" aria-label="Chat">
    <header class="flex items-center justify-between border-b px-4 py-2 dark:border-gray-800">
      <h1 class="text-1 font-semibold">Live chat</h1>
      <span class="text-5 text-sm" data-testid="viewers" title="Viewers">
        {{ chat.viewers }} {{ chat.viewers === 1 ? "viewer" : "viewers" }}
      </span>
    </header>

    <p
      v-if="notice"
      role="status"
      data-testid="chat-notice"
      class="px-4 py-2 text-sm"
      :class="
        notice.kind === 'error'
          ? 'bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-200'
          : 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-200'
      "
    >
      {{ notice.text }}
      <a v-if="notice.signIn" href="/login" class="font-semibold underline">Sign in</a>
    </p>

    <div v-if="chat.activePoll" class="px-4 pt-3">
      <ChatPollCard :poll="chat.activePoll" />
    </div>

    <div class="relative min-h-0 grow">
      <div ref="list" class="h-full overflow-y-auto px-4" data-testid="chat-list" @scroll="onScroll">
        <ul v-if="chat.messages.length > 0" class="divide-y dark:divide-gray-800">
          <ChatMessageItem v-for="message in chat.messages" :key="message.id" :message="message" />
        </ul>
        <p v-else-if="chat.loaded" class="text-5 py-8 text-center text-sm">
          No messages yet.
        </p>
      </div>

      <button
        v-if="!atBottom"
        type="button"
        class="tum-live-bg-primary tum-live-button absolute bottom-3 left-1/2 -translate-x-1/2 text-white shadow-lg"
        @click="scrollToEnd"
      >
        {{ unseen > 0 ? `${unseen} new · ` : "" }}Jump to latest
      </button>
    </div>

    <footer class="border-t p-3 dark:border-gray-800">
      <input
        type="text"
        disabled
        placeholder="Sending messages is not available here yet"
        aria-label="Message"
        class="text-5 w-full cursor-not-allowed rounded-lg border bg-gray-50 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800"
      />
    </footer>
  </section>
</template>
