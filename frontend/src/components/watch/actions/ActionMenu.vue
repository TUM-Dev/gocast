<script setup lang="ts">
import { ref } from "vue";

import { useClickOutside } from "@/lib/useClickOutside";

/** A bar button that opens a small list above it; Escape or a press outside closes it. */
defineProps<{ label: string; icon: string }>();

const open = ref(false);
const root = ref<HTMLElement | null>(null);
useClickOutside(root, () => (open.value = false));
</script>

<template>
  <div ref="root" class="relative inline-block" @keydown.escape="open = false">
    <button
      type="button"
      class="text-4 hover:text-1 flex h-8 w-8 items-center justify-center rounded"
      :title="label"
      :aria-label="label"
      aria-haspopup="menu"
      :aria-expanded="open"
      @click="open = !open"
    >
      <i class="fa-solid text-lg" :class="icon"></i>
    </button>
    <div
      v-if="open"
      role="menu"
      :aria-label="label"
      class="absolute bottom-full right-0 z-40 mb-2 grid w-64 max-w-[calc(100vw-2rem)] gap-1 rounded-lg border bg-white p-2 shadow dark:border-gray-800 dark:bg-secondary"
      @click="open = false"
    >
      <slot />
    </div>
  </div>
</template>
