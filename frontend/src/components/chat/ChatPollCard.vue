<script setup lang="ts">
import type { ChatPoll } from "@/lib/chat";
import { pollShare, pollTotalVotes } from "@/lib/chat-view";

/**
 * A poll: the question, the options and, once there are votes, their shares. While it
 * runs and the viewer may vote, the options are buttons; a vote is final. Course
 * administrators also get the button that closes it.
 */
const props = defineProps<{
  poll: ChatPoll;
  closed?: boolean;
  canVote?: boolean;
  canClose?: boolean;
  /** An action of the card is in flight. */
  busy?: boolean;
}>();

defineEmits<{ vote: [optionId: number]; close: []; dismiss: [] }>();

const votable = () => props.canVote && !props.closed && props.poll.votedOptionId === undefined;
</script>

<template>
  <section class="rounded-lg border p-3 dark:border-gray-700" aria-label="Poll" :data-poll-state="closed ? 'closed' : 'open'">
    <h2 class="text-1 text-sm font-semibold">
      {{ poll.question }}
      <span v-if="closed" class="text-5 ml-1 text-xs font-normal">(closed)</span>
    </h2>
    <ul class="mt-2 flex flex-col gap-2">
      <li v-for="option in poll.options" :key="option.id" :data-poll-option="option.id">
        <component
          :is="votable() ? 'button' : 'div'"
          :type="votable() ? 'button' : undefined"
          :disabled="votable() ? busy : undefined"
          class="text-3 flex w-full justify-between text-left text-sm"
          :class="votable() ? 'rounded border px-2 py-1 hover:bg-gray-100 dark:border-gray-700 dark:hover:bg-gray-800' : ''"
          @click="votable() && $emit('vote', option.id)"
        >
          <span>
            {{ option.answer }}
            <span v-if="poll.votedOptionId === option.id" class="text-5 text-xs">(your vote)</span>
          </span>
          <span v-if="pollTotalVotes(poll) > 0" class="text-5 tabular-nums">
            {{ option.votes }} · {{ pollShare(poll, option.votes) }}%
          </span>
        </component>
        <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
          <div
            class="h-full rounded-full bg-blue-500 dark:bg-indigo-500"
            :style="{ width: `${pollShare(poll, option.votes)}%` }"
          />
        </div>
      </li>
    </ul>
    <p v-if="votable()" class="text-5 mt-2 text-xs">Your vote is final.</p>
    <button
      v-if="canClose && !closed"
      type="button"
      class="tum-live-button mt-2 border text-sm"
      :disabled="busy"
      @click="$emit('close')"
    >
      Close poll
    </button>
    <button v-if="closed" type="button" class="text-5 mt-2 text-xs underline" @click="$emit('dismiss')">
      Dismiss results
    </button>
  </section>
</template>
