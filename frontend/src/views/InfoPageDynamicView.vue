<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";

import { isInfoPageSlug } from "@/lib/info-pages";
import InfoPageView from "@/views/InfoPageView.vue";

/**
 * Reached only when no static route matched. Confirms the slug is a page an
 * administrator has actually added before rendering it; anything else is handed back
 * to Go exactly as an unmatched path normally would be, since this component now
 * occupies the path that fallback used to own.
 *
 * A full page load never reaches this: web/course.go's shortLinkOrInfoPage makes the
 * same check server-side first, so only a client-side navigation (a link followed
 * without a reload) lands here.
 */
const route = useRoute();
const known = ref(false);

async function check(slug: string): Promise<void> {
  known.value = await isInfoPageSlug(slug);
  if (!known.value) {
    window.location.assign(route.fullPath);
  }
}

onMounted(() => check(route.params.slug as string));
watch(() => route.params.slug, (slug) => check(slug as string));
</script>

<template>
  <InfoPageView v-if="known" :name="(route.params.slug as string)" />
</template>
