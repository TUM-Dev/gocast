<script setup lang="ts">
import { MEDIA_TYPES, type CreateLectureForm, type MediaType } from "@/lib/create-lecture";

/**
 * A video upload's last step: up to one file per version, as v1 offered. They are
 * uploaded after the lecture is created, one after another.
 */
const props = defineProps<{ form: CreateLectureForm }>();

function choose(type: MediaType, event: Event): void {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (file) props.form.files[type] = file;
  else delete props.form.files[type];
}

function remove(type: MediaType): void {
  delete props.form.files[type];
  const input = document.getElementById(`create-lecture-file-${type}`) as HTMLInputElement | null;
  if (input) input.value = "";
}

const capitalized = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);
</script>

<template>
  <div class="flex flex-col gap-4 text-sm">
    <p class="text-5">
      Choose at least one: the combined video, or the presentation and camera videos separately. MP4, H.264 if possible.
    </p>
    <div v-for="{ type, label } in MEDIA_TYPES" :key="type" class="flex flex-col gap-1">
      <label :for="`create-lecture-file-${type}`" class="text-2">{{ capitalized(label) }}</label>
      <div class="flex min-w-0 flex-wrap items-center gap-2">
        <input
          :id="`create-lecture-file-${type}`"
          type="file"
          accept="video/mp4,.mp4"
          class="text-3 min-w-0 max-w-full grow text-sm"
          @change="choose(type, $event)"
        />
        <button
          v-if="form.files[type]"
          type="button"
          class="text-5 hover:text-1 text-xs"
          :aria-label="`Remove the ${label}`"
          @click="remove(type)"
        >
          <i class="fas fa-xmark mr-1"></i>Remove
        </button>
      </div>
    </div>
  </div>
</template>
