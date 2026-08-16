<script setup lang="ts">
import { onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";

import AppFooter from "@/components/AppFooter.vue";
import AppHeader from "@/components/AppHeader.vue";
import { useAuthStore } from "@/stores/auth";

const auth = useAuthStore();
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
  void auth.load();
});
</script>

<template>
  <!-- Layout copied from the body and #content wrapper of the server-rendered pages. -->
  <div class="tum-live-bg flex h-screen flex-col items-stretch">
    <AppHeader :minimal="route.meta.minimalHeader" />
    <main id="content" class="flex h-full grow justify-center overflow-y-scroll">
      <RouterView />
    </main>
    <AppFooter v-if="route.meta.footer" />
  </div>
</template>
