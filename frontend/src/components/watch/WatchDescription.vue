<script setup lang="ts">
import { computed, ref } from "vue";

import { descriptionIsLong } from "@/lib/watch";

/**
 * The lecture's description, collapsed to a few lines with "Show more" when long.
 * `html` is rendered and sanitised by the server (the same HTML the description is
 * everywhere else, and the realtime description event), so it is injected as is.
 */
const props = defineProps<{ html: string }>();

const expanded = ref(false);
const long = computed(() => descriptionIsLong(props.html));
</script>

<template>
  <section
    v-if="html.trim() !== ''"
    class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-gray-800/60"
    aria-label="Description"
    data-testid="watch-description"
  >
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div id="watch-description-text" class="text-3 prose-sm break-words" :class="{ 'line-clamp-4': long && !expanded }" v-html="html"></div>
    <button
      v-if="long"
      type="button"
      class="text-5 hover:text-1 mt-2 text-sm font-medium underline"
      :aria-expanded="expanded"
      aria-controls="watch-description-text"
      @click="expanded = !expanded"
    >
      {{ expanded ? "Show less" : "Show more" }}
    </button>
  </section>
</template>
