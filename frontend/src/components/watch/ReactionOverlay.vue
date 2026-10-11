<script setup lang="ts">
import { onBeforeUnmount, onMounted } from "vue";

import { createReactionFloats, onOwnReaction } from "@/lib/reaction-floats";
import { useStreamRealtime } from "@/stores/streamRealtime";

/**
 * The audience's reactions floating up over a live picture, as on the old live page.
 * The server sends each reaction to every viewer as just the emoji (no sender), already
 * thinned to one per emoji per second, and the viewer's own appears the moment they pick
 * it. Decoration only: hidden from assistive technology, never takes a click, and under
 * `prefers-reduced-motion` the emoji fade in place instead of travelling.
 *
 * It shares the page's realtime socket; subscribing here does not open another.
 */
const props = defineProps<{ streamId: number }>();

const realtime = useStreamRealtime(props.streamId);
const { floats, receive, own, dispose } = createReactionFloats();

let unsubscribe: (() => void) | null = null;
let stopOwn: (() => void) | null = null;
onMounted(() => {
  unsubscribe = realtime.subscribe({
    onEvent(event) {
      if (event.event.case === "streamReaction") receive(event.event.value.reaction);
    },
  });
  stopOwn = onOwnReaction(props.streamId, own);
});
onBeforeUnmount(() => {
  unsubscribe?.();
  stopOwn?.();
  dispose();
});
</script>

<template>
  <div
    class="pointer-events-none absolute inset-0 z-[5] overflow-hidden"
    aria-hidden="true"
    data-testid="reaction-overlay"
  >
    <span
      v-for="f in floats"
      :key="f.id"
      class="reaction-float absolute bottom-12 text-4xl select-none"
      :style="{ left: `${f.left}%` }"
      data-testid="reaction-float"
    >
      {{ f.emoji }}
    </span>
  </div>
</template>

<style scoped>
.reaction-float {
  animation: reaction-float-up 2.8s ease-out forwards;
}

@keyframes reaction-float-up {
  from {
    opacity: 0;
    transform: translateY(0) scale(0.8);
  }
  10% {
    opacity: 1;
  }
  to {
    opacity: 0;
    transform: translateY(-14rem) scale(1.2);
  }
}

@media (prefers-reduced-motion: reduce) {
  .reaction-float {
    animation-name: reaction-fade;
  }

  @keyframes reaction-fade {
    from,
    80% {
      opacity: 1;
    }
    to {
      opacity: 0;
    }
  }
}
</style>
