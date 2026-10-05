<script setup lang="ts">
import type { CreateLectureForm } from "@/lib/create-lecture";

/**
 * The lectures a recurring livestream makes, for review before they are created: each
 * date can be left out (except the first, which is the start itself) and given a
 * title of its own, which otherwise is the series' title.
 */
defineProps<{ form: CreateLectureForm }>();

const date = (d: Date) =>
  d.toLocaleDateString(undefined, { weekday: "short", year: "numeric", month: "short", day: "numeric" });
const time = (d: Date) => d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
</script>

<template>
  <p v-if="!form.rows.length" class="text-5 text-sm">Set the start first to see the dates.</p>
  <ol v-else class="flex max-h-96 flex-col divide-y overflow-y-auto rounded border dark:divide-gray-700 dark:border-gray-700" aria-label="Lectures of the series">
    <li
      v-for="(row, i) in form.rows"
      :key="i"
      class="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2 text-sm"
      :class="{ 'opacity-50': !row.enabled }"
    >
      <label class="text-3 flex min-w-0 basis-56 grow items-center gap-2">
        <input
          v-model="row.enabled"
          type="checkbox"
          :disabled="i === 0"
          :aria-label="`Create lecture ${i + 1}, ${date(row.start)}`"
        />
        <span class="tabular-nums">{{ i + 1 }}.</span>
        <span>{{ date(row.start) }}, {{ time(row.start) }}</span>
      </label>
      <input
        v-model="row.title"
        type="text"
        autocomplete="off"
        class="tum-live-input min-w-0 grow basis-48 py-1 text-sm"
        :aria-label="`Title of lecture ${i + 1}`"
        :placeholder="form.title || 'Same title as the series'"
        :disabled="!row.enabled"
      />
    </li>
  </ol>
</template>
