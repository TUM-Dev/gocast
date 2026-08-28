<script setup lang="ts">
import { onMounted, ref } from "vue";

import { fetchConfig } from "@/lib/config";

/**
 * The two footers are separate templates server-side — `footer` and `mobile_footer` in
 * partial/footer.gohtml — because the start page puts the mobile one inside its
 * sidebar rather than at the bottom of the page. `only` picks one half for that;
 * pages that render the footer normally take both.
 */
withDefaults(defineProps<{ only?: "desktop" | "mobile" }>(), { only: undefined });

// The templates interpolate these from IndexData. Failing to load them costs the
// version suffix and the wiki link, not the footer, so the error is swallowed.
const versionTag = ref("");
const wikiUrl = ref("");

onMounted(() => {
  fetchConfig()
    .then((config) => {
      versionTag.value = config.versionTag;
      wikiUrl.value = config.wikiUrl;
    })
    .catch(() => {});
});
</script>

<template>
  <!--
    Ported from partial/footer.gohtml. Targets are still server-rendered pages, so
    these stay plain anchors. The version and the wiki link come from GetFrontendConfig,
    which is what the template reads out of IndexData.
  -->
  <footer
    v-if="only !== 'mobile'"
    id="desktop-footer"
    class="tum-live-footer hidden items-center justify-between md:flex"
  >
    <div class="flex space-x-3">
      <a href="/about">About</a>
      <a href="/privacy">Data Privacy</a>
      <a href="/imprint">Imprint</a>
      <a v-if="wikiUrl" :href="wikiUrl" target="_blank" rel="noopener noreferrer">Wiki</a>
    </div>
    <a href="https://github.com/TUM-Dev/gocast" target="_blank" rel="noopener">
      gocast{{ versionTag ? `@${versionTag}` : "" }} <i class="fa-brands fa-github"></i>
    </a>
  </footer>

  <footer v-if="only !== 'desktop'" class="tum-live-footer w-full md:hidden">
    <div class="grid divide-y dark:divide-gray-800">
      <a href="/about">About</a>
      <a href="/privacy">Data Privacy</a>
      <a href="/imprint">Imprint</a>
    </div>
    <a
      href="https://github.com/TUM-Dev/gocast"
      target="_blank"
      rel="noopener"
      class="mt-1 block text-center"
    >
      gocast{{ versionTag ? `@${versionTag}` : "" }} <i class="fa-brands fa-github"></i>
    </a>
  </footer>
</template>
