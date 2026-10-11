<script setup lang="ts">
import type { ChatMessage } from "@/lib/chat";
import { formatMessageTime, reactionGlyph } from "@/lib/chat-view";

/**
 * One message, with its replies nested one level below it. Replies are never
 * threaded deeper: the server attaches a reply to the message it answers, and that
 * is always a top-level one.
 */
defineProps<{ message: ChatMessage; nested?: boolean }>();
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
    <p v-if="!message.visible" class="text-5 text-xs italic">Awaiting approval</p>

    <ul v-if="message.reactions.length > 0" class="mt-1 flex flex-wrap gap-1" aria-label="Reactions">
      <li
        v-for="reaction in message.reactions"
        :key="reaction.emoji"
        :title="reaction.userNames.join(', ')"
        class="text-4 flex items-center gap-1 rounded-full border px-2 text-xs dark:border-gray-700"
      >
        <span aria-hidden="true">{{ reactionGlyph(reaction.emoji) }}</span>
        <span>{{ reaction.count }}</span>
        <span class="sr-only">{{ reaction.emoji }}</span>
      </li>
    </ul>

    <ul v-if="message.replies.length > 0" class="ml-2" aria-label="Replies">
      <ChatMessageItem v-for="reply in message.replies" :key="reply.id" :message="reply" nested />
    </ul>
  </li>
</template>
