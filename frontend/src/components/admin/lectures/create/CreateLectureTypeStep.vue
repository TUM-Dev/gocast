<script setup lang="ts">
import type { CreateLectureForm, CreateMode } from "@/lib/create-lecture";

/**
 * The wizard's first step: a livestream or a video upload, and for a livestream
 * whether it is scheduled or starts now. v1 had a template for the second choice but
 * never included it in the form, so its ad-hoc path was unreachable; here it is a
 * choice like the first.
 */
const props = defineProps<{ form: CreateLectureForm }>();

interface Choice {
  value: string;
  title: string;
  detail: string;
}

const KINDS: Choice[] = [
  { value: "livestream", title: "Livestream", detail: "From a lecture hall or self-streamed" },
  { value: "vod", title: "Video upload", detail: "Upload a recording as a video on demand" },
];

const WHEN: Choice[] = [
  { value: "scheduled", title: "Schedule", detail: "One livestream or a series of them in the future" },
  { value: "adhoc", title: "Start now", detail: "A livestream starting in two minutes" },
];

function chooseKind(value: string): void {
  // A livestream starts out scheduled, the common case.
  props.form.mode = value === "vod" ? "vod" : "scheduled";
}

function chooseWhen(value: string): void {
  props.form.mode = value as CreateMode;
}

const card =
  "relative flex cursor-pointer flex-col gap-0.5 rounded-lg border p-4 hover:bg-gray-50 dark:border-gray-600 dark:hover:bg-gray-700 " +
  "has-[:checked]:border-blue-600 has-[:checked]:ring-1 has-[:checked]:ring-blue-600 dark:has-[:checked]:border-blue-400 dark:has-[:checked]:ring-blue-400 " +
  "has-[:focus-visible]:outline has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-blue-500";
</script>

<template>
  <div class="flex flex-col gap-5">
    <fieldset class="flex flex-col gap-2">
      <legend class="text-2 mb-2 text-sm font-semibold">What are you creating?</legend>
      <div class="grid gap-2 sm:grid-cols-2">
        <label v-for="kind in KINDS" :key="kind.value" :class="card">
          <input
            type="radio"
            name="create-lecture-kind"
            class="sr-only"
            :value="kind.value"
            :checked="(form.mode === 'vod') === (kind.value === 'vod')"
            @change="chooseKind(kind.value)"
          />
          <span class="text-1 font-semibold">{{ kind.title }}</span>
          <span class="text-5 text-sm">{{ kind.detail }}</span>
        </label>
      </div>
    </fieldset>

    <fieldset v-if="form.mode !== 'vod'" class="flex flex-col gap-2">
      <legend class="text-2 mb-2 text-sm font-semibold">When does it start?</legend>
      <div class="grid gap-2 sm:grid-cols-2">
        <label v-for="when in WHEN" :key="when.value" :class="card">
          <input
            type="radio"
            name="create-lecture-when"
            class="sr-only"
            :value="when.value"
            :checked="form.mode === when.value"
            @change="chooseWhen(when.value)"
          />
          <span class="text-1 font-semibold">{{ when.title }}</span>
          <span class="text-5 text-sm">{{ when.detail }}</span>
        </label>
      </div>
    </fieldset>
  </div>
</template>
