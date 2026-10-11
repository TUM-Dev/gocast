<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from "vue";

import { ChatCooldownError, type ChatMessage, type NewChatMessage } from "@/lib/chat";
import { MAX_MESSAGE_BYTES, messageBytes } from "@/lib/chat-view";

/**
 * The box a message is written in. Enter sends, Shift+Enter breaks the line.
 *
 * It owns the draft and what happens around sending: while the server cools the
 * viewer down (429) the draft stays and sending pauses a few seconds, and any other
 * refusal is shown beside the box with the draft kept, so nothing typed is lost.
 */
const props = defineProps<{
  /** Sends the message; rejects when the server refuses it. */
  send: (message: NewChatMessage) => Promise<unknown>;
  anonymousAllowed: boolean;
  /** Ordinary messages are held for an administrator to approve. */
  moderated: boolean;
  /** The message being answered. */
  replyingTo?: ChatMessage;
  /** Writing is not possible, and why; the box is shown but cannot be used. */
  disabledReason?: string;
}>();

const emit = defineEmits<{ cancelReply: []; sent: [] }>();

/** How long sending pauses after the server asks for patience. */
const COOLDOWN_PAUSE_MS = 5_000;

const text = ref("");
const anonymous = ref(false);
const sending = ref(false);
const error = ref<string | undefined>(undefined);
const paused = ref(false);
const input = ref<HTMLTextAreaElement | null>(null);
let pauseTimer: ReturnType<typeof setTimeout> | null = null;

const bytes = computed(() => messageBytes(text.value));
const tooLong = computed(() => bytes.value > MAX_MESSAGE_BYTES);
const disabled = computed(() => !!props.disabledReason);
const canSend = computed(
  () => !disabled.value && !sending.value && !paused.value && text.value.trim() !== "" && !tooLong.value,
);

async function submit(): Promise<void> {
  if (!canSend.value) return;
  sending.value = true;
  error.value = undefined;
  try {
    await props.send({
      text: text.value.trim(),
      replyTo: props.replyingTo?.id,
      anonymous: props.anonymousAllowed && anonymous.value,
    });
    text.value = "";
    emit("sent");
  } catch (err) {
    if (err instanceof ChatCooldownError) {
      error.value = err.message;
      paused.value = true;
      pauseTimer = setTimeout(() => (paused.value = false), COOLDOWN_PAUSE_MS);
    } else {
      error.value = err instanceof Error ? err.message : "The message could not be sent.";
    }
  } finally {
    sending.value = false;
  }
}

function onKeydown(e: KeyboardEvent): void {
  if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
    e.preventDefault();
    void submit();
  }
}

onBeforeUnmount(() => pauseTimer !== null && clearTimeout(pauseTimer));

defineExpose({ focus: () => input.value?.focus() });
</script>

<template>
  <form class="flex flex-col gap-2" aria-label="Write a message" @submit.prevent="submit">
    <p v-if="replyingTo" class="text-5 flex items-center justify-between rounded bg-gray-100 px-2 py-1 text-xs dark:bg-gray-800" data-testid="replying-to">
      <span class="truncate">Replying to {{ replyingTo.userName }}</span>
      <button type="button" class="ml-2 underline" @click="emit('cancelReply')">Cancel reply</button>
    </p>

    <p v-if="moderated && !disabled" class="text-5 text-xs">
      Messages are reviewed by the course's administrators before others see them.
    </p>

    <label class="sr-only" for="chat-message">Message</label>
    <textarea
      id="chat-message"
      ref="input"
      v-model="text"
      rows="2"
      :disabled="disabled"
      :placeholder="disabledReason ?? 'Write a message'"
      :aria-invalid="tooLong"
      aria-describedby="chat-message-hint"
      class="text-3 w-full resize-none rounded-lg border bg-white px-3 py-2 text-sm disabled:cursor-not-allowed disabled:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:disabled:bg-gray-800"
      @keydown="onKeydown"
    />

    <p v-if="error" role="alert" data-testid="composer-error" class="text-sm text-red-700 dark:text-red-300">
      {{ error }}
    </p>

    <div class="flex items-center justify-between gap-2">
      <label v-if="anonymousAllowed" class="text-4 flex items-center gap-1 text-sm">
        <input v-model="anonymous" type="checkbox" :disabled="disabled" />
        Send anonymously
      </label>
      <span v-else />
      <span id="chat-message-hint" class="text-5 text-xs tabular-nums" :class="{ 'text-red-700 dark:text-red-300': tooLong }">
        {{ bytes }}/{{ MAX_MESSAGE_BYTES }}
      </span>
      <button
        type="submit"
        class="tum-live-bg-primary tum-live-button text-white disabled:opacity-50"
        :disabled="!canSend"
      >
        {{ paused ? "Wait a moment" : "Send" }}
      </button>
    </div>
  </form>
</template>
