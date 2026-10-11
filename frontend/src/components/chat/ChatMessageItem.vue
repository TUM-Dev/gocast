<script setup lang="ts">
import { computed, inject, nextTick, ref } from "vue";

import type { ChatMessage } from "@/lib/chat";
import { REACTIONS, formatMessageTime, reactionGlyph, reactionName } from "@/lib/chat-view";
import { chatContextKey } from "./chat-context";

/**
 * One message, with its replies nested one level below it. Replies are never
 * threaded deeper: the server attaches a reply to the message it answers, and that
 * is always a top-level one.
 *
 * What can be done to a message comes from the panel's context; without one the
 * message is read-only.
 */
const props = defineProps<{ message: ChatMessage; nested?: boolean }>();

const context = inject(chatContextKey, null);
const ctx = computed(() => context?.() ?? null);

const canWrite = computed(() => !!ctx.value?.access.canWrite && !ctx.value.writeBlocked);
const canModerate = computed(() => !!ctx.value?.access.canModerate);

const pickerOpen = ref(false);
const confirmingDelete = ref(false);
const pickerButton = ref<HTMLButtonElement | null>(null);

function mine(emoji: string): boolean {
  const id = ctx.value?.userId;
  return id !== undefined && !!props.message.reactions.find((r) => r.emoji === emoji)?.userIds.includes(id);
}

function react(emoji: string): void {
  ctx.value?.react(props.message, emoji);
}

function pick(emoji: string): void {
  react(emoji);
  closePicker();
}

async function closePicker(): Promise<void> {
  pickerOpen.value = false;
  await nextTick();
  pickerButton.value?.focus();
}
</script>

<template>
  <li
    :data-message-id="message.id"
    :class="[nested ? 'mt-2 border-l-2 pl-3 dark:border-gray-700' : 'py-2']"
  >
    <div class="flex items-baseline gap-2 text-sm">
      <!-- The colour is the server's pick per author, not a theme colour. -->
      <span class="font-semibold" :style="{ color: message.color || undefined }">{{
        message.userName
      }}</span>
      <span
        v-if="message.admin"
        class="rounded bg-blue-100 px-1 text-xs font-semibold text-blue-700 dark:bg-indigo-900 dark:text-indigo-200"
        >Admin</span
      >
      <time class="text-5 text-xs" :datetime="message.createdAt.toISOString()">{{
        formatMessageTime(message.createdAt)
      }}</time>
      <span v-if="message.resolved" class="text-xs text-green-600 dark:text-green-400">Resolved</span>
    </div>

    <!-- `html` is sanitised by the server, links included; `text` is never rendered as HTML. -->
    <!-- eslint-disable-next-line vue/no-v-html -->
    <p class="text-3 break-words text-sm" :class="{ 'opacity-60': !message.visible }" v-html="message.html" />
    <p v-if="!message.visible" class="text-5 text-xs italic">
      {{ message.retracted ? "Retracted" : "Awaiting approval" }}
    </p>

    <ul v-if="message.reactions.length > 0" class="mt-1 flex flex-wrap gap-1" aria-label="Reactions">
      <li v-for="reaction in message.reactions" :key="reaction.emoji">
        <button
          v-if="canWrite && message.visible"
          type="button"
          :title="reaction.userNames.join(', ')"
          :aria-pressed="mine(reaction.emoji)"
          :aria-label="`${reactionName(reaction.emoji)}, ${reaction.count}`"
          class="text-4 flex items-center gap-1 rounded-full border px-2 text-xs hover:bg-gray-100 dark:border-gray-700 dark:hover:bg-gray-800"
          :class="{ 'border-blue-500 bg-blue-50 dark:border-indigo-400 dark:bg-indigo-950': mine(reaction.emoji) }"
          @click="react(reaction.emoji)"
        >
          <span aria-hidden="true">{{ reactionGlyph(reaction.emoji) }}</span>
          <span aria-hidden="true">{{ reaction.count }}</span>
        </button>
        <span
          v-else
          :title="reaction.userNames.join(', ')"
          class="text-4 flex items-center gap-1 rounded-full border px-2 text-xs dark:border-gray-700"
        >
          <span aria-hidden="true">{{ reactionGlyph(reaction.emoji) }}</span>
          <span>{{ reaction.count }}</span>
          <span class="sr-only">{{ reaction.emoji }}</span>
        </span>
      </li>
    </ul>

    <!-- Actions. Moderation is offered to administrators only; the server checks it all the same. -->
    <div
      v-if="ctx && (canWrite || canModerate)"
      class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs"
      role="group"
      aria-label="Message actions"
    >
      <div v-if="canWrite && message.visible" class="relative">
        <button
          ref="pickerButton"
          type="button"
          class="text-5 hover:underline"
          aria-haspopup="true"
          :aria-expanded="pickerOpen"
          @click="pickerOpen = !pickerOpen"
        >
          React
        </button>
        <div
          v-if="pickerOpen"
          role="group"
          aria-label="Pick a reaction"
          class="absolute left-0 z-10 mt-1 flex gap-1 rounded-lg border bg-white p-1 shadow-lg dark:border-gray-700 dark:bg-gray-900"
          @keydown.esc.stop="closePicker"
        >
          <button
            v-for="emoji in REACTIONS"
            :key="emoji"
            type="button"
            class="rounded p-1 text-base hover:bg-gray-100 dark:hover:bg-gray-800"
            :aria-label="reactionName(emoji)"
            :aria-pressed="mine(emoji)"
            @click="pick(emoji)"
          >
            <span aria-hidden="true">{{ reactionGlyph(emoji) }}</span>
          </button>
        </div>
      </div>
      <button
        v-if="canWrite && !nested && message.visible"
        type="button"
        class="text-5 hover:underline"
        @click="ctx.reply(message)"
      >
        Reply
      </button>

      <template v-if="canModerate">
        <button
          v-if="!message.visible"
          type="button"
          class="font-semibold text-green-700 hover:underline dark:text-green-400"
          @click="ctx.approve(message)"
        >
          {{ message.retracted ? "Publish again" : "Approve" }}
        </button>
        <button v-else type="button" class="text-5 hover:underline" @click="ctx.retract(message)">
          Retract
        </button>
        <button
          v-if="!message.resolved"
          type="button"
          class="text-5 hover:underline"
          @click="ctx.resolve(message)"
        >
          Resolve
        </button>
        <button
          v-if="!confirmingDelete"
          type="button"
          class="text-red-600 hover:underline dark:text-red-400"
          @click="confirmingDelete = true"
        >
          Delete
        </button>
        <span v-else class="flex items-center gap-2" role="group" aria-label="Confirm deleting">
          <button
            type="button"
            class="font-semibold text-red-600 hover:underline dark:text-red-400"
            @click="ctx.remove(message)"
          >
            Confirm delete
          </button>
          <button type="button" class="text-5 hover:underline" @click="confirmingDelete = false">
            Cancel
          </button>
        </span>
      </template>
    </div>

    <ul v-if="message.replies.length > 0" class="ml-2" aria-label="Replies">
      <ChatMessageItem v-for="reply in message.replies" :key="reply.id" :message="reply" nested />
    </ul>
  </li>
</template>
