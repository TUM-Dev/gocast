<script setup lang="ts">
import { nextTick, ref, watch } from "vue";

import { useClickOutside } from "@/lib/useClickOutside";

/**
 * A bar button that opens a small list above it; Escape or a press outside closes it.
 *
 * The list is as wide as 16rem and right-aligned to its button, which is right for a
 * button at the end of the bar and runs off the left edge of the screen for one near
 * the start (the download and view buttons sit there). So the list is measured when it
 * opens and nudged back inside the viewport; if there is no room above (a bar scrolled
 * to the top of a short window) it opens below instead.
 */
defineProps<{ label: string; icon: string }>();

const open = ref(false);
const root = ref<HTMLElement | null>(null);
useClickOutside(root, () => (open.value = false));

const MARGIN = 8;
const menu = ref<HTMLElement | null>(null);
const shiftX = ref(0);
const below = ref(false);

watch(open, async (isOpen) => {
  shiftX.value = 0;
  below.value = false;
  if (!isOpen) return;
  await nextTick();
  const el = menu.value;
  if (!el) return;
  const r = el.getBoundingClientRect();
  const vw = document.documentElement.clientWidth;
  if (r.left < MARGIN) shiftX.value = MARGIN - r.left;
  else if (r.right > vw - MARGIN) shiftX.value = vw - MARGIN - r.right;
  below.value = r.top < MARGIN;
});
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
      ref="menu"
      role="menu"
      :aria-label="label"
      :style="{ transform: shiftX ? `translateX(${shiftX}px)` : undefined }"
      :class="below ? 'top-full mt-2' : 'bottom-full mb-2'"
      class="absolute right-0 z-40 grid w-64 max-w-[calc(100vw-2rem)] gap-1 rounded-lg border bg-white p-2 shadow dark:border-gray-800 dark:bg-secondary"
      @click="open = false"
    >
      <slot />
    </div>
  </div>
</template>
