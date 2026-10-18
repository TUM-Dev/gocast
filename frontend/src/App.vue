<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { START_LOCATION, useRoute, useRouter } from "vue-router";

import AppFooter from "@/components/AppFooter.vue";
import AppHeader from "@/components/AppHeader.vue";
import DownloadNotice from "@/components/DownloadNotice.vue";
import { readServerError } from "@/lib/server-error";
import { isEmbedQuery } from "@/lib/watch-admin-embed";
import { useAuthStore } from "@/stores/auth";
import { useSidenavStore } from "@/stores/sidenav";
import ErrorView from "@/views/ErrorView.vue";

const auth = useAuthStore();
const sidenav = useSidenavStore();
const route = useRoute();
const router = useRouter();

// The popup chat, and the embedded player Artemis frames (`?video_only`), are windows of
// their own: no header, no footer.
const bare = computed(() => !!route.meta.bare || (isEmbedQuery(route.query) && route.name === "watch") || (isEmbedQuery(route.query) && route.name === "watch-layout"));

// Set when Go refused this page load (403, 404, ...): the error is shown instead of the
// route, until the user navigates anywhere else in the app.
const serverError = ref(readServerError());
router.afterEach((_to, from) => {
  if (from !== START_LOCATION) serverError.value = null;
});

// The header renders on every page and needs to know who is signed in, so the user is
// loaded here rather than by each view. The store only fetches once.
onMounted(async () => {
  // The first navigation resolves after mounting, so route.meta is still empty here
  // until the router is ready — reading it any earlier reports every page as public.
  await router.isReady();

  // Nothing to load on a page reached before signing in: there the token request can
  // only fail, and a 401 in the console on the login page reads as a broken deployment.
  if (route.meta.anonymous) return;
  // Fire and forget: the header is decoration, and the store leaves itself unloaded
  // when the request fails for anything but a 401, so the next caller retries. Caught
  // all the same — an uncaught rejection here is a console error on every page load
  // while the API is down.
  auth.load().catch(() => {});
});
</script>

<template>
  <div class="tum-live-bg flex h-screen flex-col items-stretch">
    <AppHeader
      v-if="!bare"
      :minimal="route.meta.minimalHeader"
      :show-sidenav-toggle="route.meta.sidenav"
      @toggle-sidenav="sidenav.toggle()"
    />
    <main id="content" class="flex h-full grow justify-center overflow-y-scroll">
      <ErrorView v-if="serverError" :status="serverError.status" :message="serverError.message" />
      <RouterView v-else />
    </main>
    <!--
      Where the page has the start page's sidebar, the mobile footer is rendered
      inside it instead — which is how the templates arrange it, `footer` and
      `mobile_footer` being separate partials for exactly this reason.
    -->
    <AppFooter v-if="route.meta.footer && !bare" :only="route.meta.sidenav ? 'desktop' : undefined" />
    <DownloadNotice />
  </div>
</template>
