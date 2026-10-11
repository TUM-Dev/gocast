<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";

import WatchPage from "@/components/watch/WatchPage.vue";
import { isEmbedQuery } from "@/lib/watch-admin-embed";
import { singleRouteParam } from "@/lib/route-query";
import NotFoundView from "@/views/NotFoundView.vue";
import WatchEmbedView from "@/views/WatchEmbedView.vue";

/**
 * `/w/:slug/:streamID` and `/w/:slug/:streamID/:version` (COMB, PRES, CAM, SPLIT).
 *
 * A thin shell around WatchPage that keys it by stream: moving from one lecture to
 * another (the playlist) is a new page, not a re-render of the old one, so the chat
 * store, the player and the pending polls of the previous lecture are torn down with
 * it. A change of `:version` alone keeps the page and swaps the layout.
 */
const route = useRoute();
const slug = computed(() => singleRouteParam(route.params, "slug"));
const streamId = computed(() => Number(singleRouteParam(route.params, "streamID")));
const valid = computed(() => !!slug.value && Number.isInteger(streamId.value) && streamId.value > 0);
// Artemis frames the player alone: the same URL with `?video_only=1`.
const embed = computed(() => isEmbedQuery(route.query));
</script>

<template>
  <WatchEmbedView v-if="valid && embed" :key="`embed/${slug}/${streamId}`" />
  <WatchPage v-else-if="valid" :key="`${slug}/${streamId}`" :slug="slug!" :stream-id="streamId" />
  <NotFoundView v-else />
</template>
