<script setup lang="ts">
import { computed, ref } from "vue";

/**
 * The form an administrator starts a poll with: a question and two to ten answers.
 * The limits are the server's; it checks them all the same.
 */
const props = defineProps<{ busy?: boolean; error?: string }>();
const emit = defineEmits<{ submit: [question: string, answers: string[]]; cancel: [] }>();

const MIN_ANSWERS = 2;
const MAX_ANSWERS = 10;

const question = ref("");
const answers = ref<string[]>(["", ""]);

const valid = computed(
  () => question.value.trim() !== "" && answers.value.every((a) => a.trim() !== ""),
);

function submit(): void {
  if (!valid.value || props.busy) return;
  emit("submit", question.value.trim(), answers.value.map((a) => a.trim()));
}
</script>

<template>
  <form
    class="flex flex-col gap-2 rounded-lg border p-3 dark:border-gray-700"
    aria-label="Start a poll"
    @submit.prevent="submit"
  >
    <label class="text-sm font-semibold">
      Question
      <input
        v-model="question"
        type="text"
        maxlength="500"
        required
        class="text-3 mt-1 w-full rounded border bg-white px-2 py-1 text-sm font-normal dark:border-gray-700 dark:bg-gray-900"
      />
    </label>
    <div v-for="(_, i) in answers" :key="i" class="flex items-center gap-2">
      <label class="grow text-sm">
        <span class="sr-only">Answer {{ i + 1 }}</span>
        <input
          v-model="answers[i]"
          type="text"
          maxlength="200"
          required
          :placeholder="`Answer ${i + 1}`"
          :aria-label="`Answer ${i + 1}`"
          class="text-3 w-full rounded border bg-white px-2 py-1 text-sm dark:border-gray-700 dark:bg-gray-900"
        />
      </label>
      <button
        v-if="answers.length > MIN_ANSWERS"
        type="button"
        class="text-5 text-xs underline"
        :aria-label="`Remove answer ${i + 1}`"
        @click="answers.splice(i, 1)"
      >
        Remove
      </button>
    </div>
    <button
      v-if="answers.length < MAX_ANSWERS"
      type="button"
      class="text-5 self-start text-xs underline"
      @click="answers.push('')"
    >
      Add an answer
    </button>
    <p v-if="error" role="alert" class="text-sm text-red-700 dark:text-red-300">{{ error }}</p>
    <div class="flex gap-2">
      <button type="submit" class="tum-live-bg-primary tum-live-button text-white" :disabled="!valid || busy">
        Start poll
      </button>
      <button type="button" class="tum-live-button border" @click="$emit('cancel')">Cancel</button>
    </div>
  </form>
</template>
