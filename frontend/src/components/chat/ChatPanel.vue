<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, provide, ref, watch } from "vue";

import ChatComposer from "@/components/chat/ChatComposer.vue";
import ChatMessageItem from "@/components/chat/ChatMessageItem.vue";
import ChatPollCard from "@/components/chat/ChatPollCard.vue";
import ChatPollForm from "@/components/chat/ChatPollForm.vue";
import { chatContextKey, type ChatContext } from "@/components/chat/chat-context";
import { ApiError } from "@/lib/api";
import type { ChatMessage, NewChatMessage } from "@/lib/chat";
import {
  connectionNotice,
  countAwaitingApproval,
  countMessages,
  shownMessages,
} from "@/lib/chat-view";
import { useAuthStore } from "@/stores/auth";
import { useChatStore } from "@/stores/chat";

/**
 * A stream's chat: messages with their replies, the running poll, the viewer count
 * and the state of the connection, and now the means to write, react and vote — and,
 * for the course's administrators, to moderate and run polls. Everything comes from
 * the chat store, which the realtime socket keeps current; this draws it and routes
 * the actions. What the viewer may do is the server's answer in `chat.access`; the
 * server checks every action again.
 */
const props = defineProps<{ streamId: number }>();

// A different stream is a different store, so the panel is remounted per stream (the
// view keys it) rather than switching stores in place.
const chat = useChatStore(props.streamId);
const auth = useAuthStore();

const notice = computed(() =>
  connectionNotice({
    status: chat.status,
    loaded: chat.loaded,
    error: chat.error,
    fatalCode: chat.fatalCode,
    lastCloseCode: chat.lastCloseCode,
  }),
);

const userId = computed(() => auth.user?.id);
const canModerate = computed(() => chat.access.canModerate);
const shown = computed(() => shownMessages(chat.messages, canModerate.value, userId.value));
const total = computed(() => countMessages(shown.value));
const awaiting = computed(() => (canModerate.value ? countAwaitingApproval(chat.messages) : 0));

/** Why the box cannot be used, or undefined when it can. */
const disabledReason = computed(() => {
  if (chat.sessionExpired) return "Your session has expired. Sign in to write.";
  if (!chat.loaded || chat.access.canWrite) return undefined;
  return auth.loaded && !auth.user ? "Sign in to write in the chat." : "The chat is switched off.";
});
const showSignIn = computed(() => disabledReason.value !== undefined && (chat.sessionExpired || !auth.user));

// ----- Actions -----

const replyingTo = ref<ChatMessage | undefined>(undefined);
const composer = ref<InstanceType<typeof ChatComposer> | null>(null);
/** The last action that the server refused, shown until the next one succeeds. */
const actionError = ref<string | undefined>(undefined);
const pollBusy = ref(false);
const pollFormOpen = ref(false);
const pollFormError = ref<string | undefined>(undefined);

const errorText = (err: unknown): string => (err instanceof Error ? err.message : String(err));

async function run(action: () => Promise<unknown>): Promise<void> {
  try {
    await action();
    actionError.value = undefined;
  } catch (err) {
    actionError.value = errorText(err);
  }
}

const send = (message: NewChatMessage) => chat.sendMessage(message);

async function reply(message: ChatMessage): Promise<void> {
  replyingTo.value = message;
  await nextTick();
  composer.value?.focus();
}

async function startPoll(question: string, answers: string[]): Promise<void> {
  pollBusy.value = true;
  pollFormError.value = undefined;
  try {
    await chat.openPoll(question, answers);
    pollFormOpen.value = false;
  } catch (err) {
    pollFormError.value =
      err instanceof ApiError && err.status === 409
        ? "A poll is running already. Close it first."
        : errorText(err);
  } finally {
    pollBusy.value = false;
  }
}

async function pollAction(action: () => Promise<unknown>): Promise<void> {
  pollBusy.value = true;
  await run(action);
  pollBusy.value = false;
}

// A getter, so a message always reads the current access and user.
provide(
  chatContextKey,
  (): ChatContext => ({
    access: chat.access,
    userId: userId.value,
    writeBlocked: chat.sessionExpired,
    react: (m, emoji) => void run(() => chat.toggleReaction(m.id, emoji)),
    reply: (m) => void reply(m),
    approve: (m) => void run(() => chat.approve(m.id)),
    retract: (m) => void run(() => chat.retract(m.id)),
    remove: (m) =>
      void run(async () => {
        await chat.remove(m.id);
        if (replyingTo.value?.id === m.id) replyingTo.value = undefined;
      }),
    resolve: (m) => void run(() => chat.resolve(m.id)),
  }),
);

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

onMounted(() => {
  chat.start();
  // Who the viewer is only decorates (own reactions, the sign-in hint); a failure is the page's to report.
  void auth.load().catch(() => undefined);
});
onBeforeUnmount(() => chat.stop());
</script>

<template>
  <section class="flex h-full min-h-0 w-full flex-col" aria-label="Chat">
    <header class="flex items-center justify-between gap-2 border-b px-4 py-2 dark:border-gray-800">
      <h1 class="text-1 font-semibold">Live chat</h1>
      <span class="flex items-center gap-3">
        <span
          v-if="awaiting > 0"
          class="rounded-full bg-amber-100 px-2 text-xs font-semibold text-amber-800 dark:bg-amber-900 dark:text-amber-100"
          data-testid="awaiting"
        >
          {{ awaiting }} awaiting approval
        </span>
        <span class="text-5 text-sm" data-testid="viewers" title="Viewers">
          {{ chat.viewers }} {{ chat.viewers === 1 ? "viewer" : "viewers" }}
        </span>
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

    <p
      v-if="actionError"
      role="alert"
      data-testid="action-error"
      class="flex items-center justify-between gap-2 bg-red-50 px-4 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-200"
    >
      <span>{{ actionError }}</span>
      <button type="button" class="underline" @click="actionError = undefined">Dismiss</button>
    </p>

    <div v-if="chat.activePoll || chat.closedPoll || canModerate" class="flex flex-col gap-2 px-4 pt-3">
      <ChatPollCard
        v-if="chat.activePoll"
        :poll="chat.activePoll"
        :can-vote="chat.access.canWrite && !chat.sessionExpired"
        :can-close="canModerate"
        :busy="pollBusy"
        @vote="(id) => pollAction(() => chat.vote(id))"
        @close="pollAction(() => chat.endPoll())"
      />
      <ChatPollCard
        v-else-if="chat.closedPoll"
        :poll="chat.closedPoll"
        closed
        @dismiss="chat.dismissClosedPoll()"
      />
      <ChatPollForm
        v-if="canModerate && !chat.activePoll && pollFormOpen"
        :busy="pollBusy"
        :error="pollFormError"
        @submit="startPoll"
        @cancel="pollFormOpen = false"
      />
      <button
        v-else-if="canModerate && !chat.activePoll"
        type="button"
        class="text-5 self-start text-sm underline"
        @click="pollFormOpen = true"
      >
        Start a poll
      </button>
    </div>

    <div class="relative min-h-0 grow">
      <div ref="list" class="h-full overflow-y-auto px-4" data-testid="chat-list" @scroll="onScroll">
        <ul v-if="shown.length > 0" class="divide-y dark:divide-gray-800">
          <ChatMessageItem v-for="message in shown" :key="message.id" :message="message" />
        </ul>
        <p v-else-if="chat.loaded" class="text-5 py-8 text-center text-sm">
          No messages yet.
        </p>
      </div>

      <button
        v-if="!atBottom"
        type="button"
        data-testid="jump-latest"
        class="tum-live-bg-primary tum-live-button absolute bottom-3 left-1/2 -translate-x-1/2 text-white shadow-lg"
        @click="scrollToEnd"
      >
        {{ unseen > 0 ? `${unseen} new · ` : "" }}Jump to latest
      </button>
    </div>

    <footer class="border-t p-3 dark:border-gray-800">
      <p v-if="showSignIn" class="text-5 mb-2 text-sm">
        <a href="/login" class="font-semibold underline">Sign in</a> to write in the chat.
      </p>
      <ChatComposer
        ref="composer"
        :send="send"
        :anonymous-allowed="chat.access.anonymousAllowed"
        :moderated="chat.access.moderated && !canModerate"
        :replying-to="replyingTo"
        :disabled-reason="disabledReason"
        @cancel-reply="replyingTo = undefined"
        @sent="replyingTo = undefined"
      />
    </footer>
  </section>
</template>
