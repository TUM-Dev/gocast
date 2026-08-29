<script setup lang="ts">
import { onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";

import AppFooter from "@/components/AppFooter.vue";
import AppHeader from "@/components/AppHeader.vue";
import { useAuthStore } from "@/stores/auth";
import { useSidenavStore } from "@/stores/sidenav";

const auth = useAuthStore();
const sidenav = useSidenavStore();
const route = useRoute();
const router = useRouter();

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
  <!-- Layout copied from the body and #content wrapper of the server-rendered pages. -->
  <div class="tum-live-bg flex h-screen flex-col items-stretch">
    <AppHeader
      :minimal="route.meta.minimalHeader"
      :show-sidenav-toggle="route.meta.sidenav"
      @toggle-sidenav="sidenav.toggle()"
    />
    <main id="content" class="flex h-full grow justify-center overflow-y-scroll">
      <RouterView />
    </main>
    <!--
      Where the page has the start page's sidebar, the mobile footer is rendered
      inside it instead — which is how the templates arrange it, `footer` and
      `mobile_footer` being separate partials for exactly this reason.
    -->
    <AppFooter v-if="route.meta.footer" :only="route.meta.sidenav ? 'desktop' : undefined" />
  </div>
</template>
