<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";

import { isInfoPageSlug } from "@/lib/info-pages";
import { isInitialNavigation } from "@/router";
import InfoPageView from "@/views/InfoPageView.vue";
import NotFoundView from "@/views/NotFoundView.vue";

/**
 * Reached only when no static route matched. Confirms the slug is a page an
 * administrator has actually added before rendering it. Anything else is handed back
 * to Go exactly as an unmatched path normally would be, since this component now
 * occupies the path that fallback used to own -- unless this is the page load itself:
 * then web/course.go's shortLinkOrInfoPage has already made the same check, found
 * neither a page nor a course short link, and answered with the shell and a 404, so
 * the not-found page is shown rather than reloading forever.
 */
const route = useRoute();
const known = ref(false);
const missing = ref(false);

async function check(slug: string): Promise<void> {
  const initial = isInitialNavigation();
  known.value = await isInfoPageSlug(slug);
  if (known.value) {
    missing.value = false;
  } else if (initial) {
    missing.value = true;
  } else {
    window.location.assign(route.fullPath);
  }
}

onMounted(() => check(route.params.slug as string));
watch(() => route.params.slug, (slug) => check(slug as string));
</script>

<template>
  <InfoPageView v-if="known" :name="(route.params.slug as string)" />
  <NotFoundView v-else-if="missing" />
</template>
