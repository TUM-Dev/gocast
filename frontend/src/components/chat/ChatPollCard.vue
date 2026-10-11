<script setup lang="ts">
import type { ChatPoll } from "@/lib/chat";
import { pollShare, pollTotalVotes } from "@/lib/chat-view";

/**
 * The running poll, read-only: the question, the options and, once there are votes,
 * their shares. Voting arrives with the write RPCs.
 */
defineProps<{ poll: ChatPoll; closed?: boolean }>();
</script>

<template>
  <section class="rounded-lg border p-3 dark:border-gray-700" aria-label="Poll">
    <h2 class="text-1 text-sm font-semibold">
      {{ poll.question }}
      <span v-if="closed" class="text-5 ml-1 text-xs font-normal">(closed)</span>
    </h2>
    <ul class="mt-2 flex flex-col gap-2">
      <li v-for="option in poll.options" :key="option.id" :data-poll-option="option.id">
        <div class="text-3 flex justify-between text-sm">
          <span>
            {{ option.answer }}
            <span v-if="poll.votedOptionId === option.id" class="text-5 text-xs">(your vote)</span>
          </span>
          <span v-if="pollTotalVotes(poll) > 0" class="text-5 tabular-nums">
            {{ option.votes }} · {{ pollShare(poll, option.votes) }}%
          </span>
        </div>
        <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
          <div
            class="h-full rounded-full bg-blue-500 dark:bg-indigo-500"
            :style="{ width: `${pollShare(poll, option.votes)}%` }"
          />
        </div>
      </li>
    </ul>
  </section>
</template>
