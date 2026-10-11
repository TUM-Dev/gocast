<script setup lang="ts">
import { redirectToLogin } from "@/stores/auth";

/**
 * A full-page reason the stream cannot be shown: private (signed in but not allowed),
 * login (anonymous, and signing in may help) or a failed load.
 */
defineProps<{ kind: "private" | "login" | "error" }>();
defineEmits<{ retry: [] }>();
</script>

<template>
  <section class="text-3 mx-auto flex w-full max-w-2xl flex-col items-center gap-4 p-6 text-center" data-testid="watch-message">
    <template v-if="kind === 'private'">
      <h1 class="text-1 text-2xl font-semibold">You may not watch this lecture.</h1>
      <p class="text-5 text-sm">It is private, or belongs to a course you are not enrolled in.</p>
    </template>
    <template v-else-if="kind === 'login'">
      <h1 class="text-1 text-2xl font-semibold">Sign in to watch this lecture.</h1>
      <p class="text-5 text-sm">This lecture is only available to signed-in users.</p>
      <button type="button" class="tum-live-button tum-live-button-primary px-4 py-2" @click="redirectToLogin()">Sign in</button>
    </template>
    <template v-else>
      <h1 class="text-1 text-2xl font-semibold">The lecture could not be loaded.</h1>
      <p class="text-5 text-sm">Something went wrong on our side. Please try again.</p>
      <button type="button" class="tum-live-button tum-live-button-primary px-4 py-2" @click="$emit('retry')">Try again</button>
    </template>
  </section>
</template>
