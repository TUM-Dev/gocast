<script setup lang="ts">
import { nextTick, ref, watch } from "vue";

/**
 * A modal for the action bar's forms. Teleported to <body> so the action bar's
 * overflow and stacking never clip it; Escape and a click on the backdrop close it, and
 * focus moves in on open and back to the opener on close.
 */
const props = defineProps<{ open: boolean; title: string }>();
const emit = defineEmits<{ close: [] }>();

const panel = ref<HTMLElement | null>(null);
let opener: HTMLElement | null = null;

watch(
  () => props.open,
  async (open) => {
    if (open) {
      opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      await nextTick();
      panel.value?.focus();
    } else {
      opener?.focus();
      opener = null;
    }
  },
);
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      data-testid="action-dialog"
      @click.self="emit('close')"
      @keydown.escape.stop="emit('close')"
    >
      <div
        ref="panel"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
        tabindex="-1"
        class="text-3 max-h-full w-full max-w-md overflow-y-auto rounded-lg border bg-white shadow-lg outline-none dark:border-gray-800 dark:bg-secondary-light"
      >
        <div class="flex items-center justify-between px-4 pb-2 pt-4">
          <h2 class="text-3 text-base font-semibold">{{ title }}</h2>
          <button
            type="button"
            title="Close"
            aria-label="Close"
            class="rounded-full p-2 hover:bg-gray-100 dark:hover:bg-gray-600"
            @click="emit('close')"
          >
            <i class="fa-solid fa-xmark text-3"></i>
          </button>
        </div>
        <div class="px-4 pb-4"><slot /></div>
      </div>
    </div>
  </Teleport>
</template>
