<script setup lang="ts">
/**
 * Stands where another agent's component is not wired in yet. The integrator deletes
 * each use together with the TODO beside it.
 */
defineProps<{ label: string }>();
</script>

<template>
  <p class="text-5 p-3 text-sm italic" data-testid="watch-placeholder">{{ label }}</p>
</template>
