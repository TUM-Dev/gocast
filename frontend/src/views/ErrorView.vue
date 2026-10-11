<script setup lang="ts">
import { RouterLink } from "vue-router";

/** A page request the server refused: its status and the reason it gave. */
defineProps<{ status: number; message: string }>();
</script>

<template>
  <section class="text-3 mx-auto flex w-full max-w-2xl flex-col items-center gap-4 p-6 text-center">
    <p class="text-5 text-6xl font-bold" aria-hidden="true">{{ status }}</p>
    <h1 class="text-1 text-2xl font-semibold">{{ message }}</h1>
    <RouterLink to="/" class="tum-live-button tum-live-button-primary px-4 py-2">Back to the start page</RouterLink>
  </section>
</template>
