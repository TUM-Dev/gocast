<script setup lang="ts">
import { RouterLink } from "vue-router";

/**
 * The 404 page. Go serves the shell with that status for a path nothing owns, and
 * the router's catch-all lands here; the dynamic info-page route renders it too when
 * a first load names a page nobody added. The template it replaces said the same
 * words, which the browser tests look for.
 */
</script>

<template>
  <section class="text-3 mx-auto flex w-full max-w-2xl flex-col items-center gap-4 p-6 text-center">
    <p class="text-5 text-6xl font-bold" aria-hidden="true">404</p>
    <h1 class="text-1 text-2xl font-semibold">This page does not exist.</h1>
    <p class="text-5 text-sm">
      The link may be out of date, or the page may have moved. If you followed a link on this
      site, please let us know.
    </p>
    <RouterLink to="/" class="tum-live-button tum-live-button-primary px-4 py-2">Back to the start page</RouterLink>
  </section>
</template>
