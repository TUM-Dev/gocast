<script setup lang="ts">
import { onMounted, ref, watch } from "vue";

import { fetchInfoPage, type InfoPageName } from "@/lib/info-pages";

const props = defineProps<{ name: InfoPageName }>();

const content = ref("");
const failed = ref(false);
const loaded = ref(false);

async function load(name: InfoPageName): Promise<void> {
  loaded.value = false;
  failed.value = false;
  try {
    content.value = await fetchInfoPage(name);
  } catch {
    // A 404 and a fault read the same to a visitor, so they are not distinguished.
    failed.value = true;
    content.value = "";
  } finally {
    loaded.value = true;
  }
}

onMounted(() => load(props.name));
// The three routes share this component, so only the prop changes between them.
watch(() => props.name, load);
</script>

<template>
  <!-- Widths copied from web/template/info-page.gohtml, so the layout is unchanged. -->
  <div class="text-3 mx-auto w-full p-6 md:w-1/2 2xl:max-w-(--breakpoint-xl)">
    <!-- No heading of its own: the Markdown opens with one, as in the template. -->
    <!-- v-html is safe only because bluemonday already ran server-side; do not
         sanitise again here, two filters disagreeing is worse than one. -->
    <div v-if="content" class="tum-live-markdown pb-10" v-html="content"></div>
    <p v-else-if="loaded && failed" class="text-5 pb-10">
      This page is not available.
    </p>
    <p v-else-if="loaded" class="text-5 pb-10">This page has not been written yet.</p>
  </div>
</template>
