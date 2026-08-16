<script setup lang="ts">
import { useAuthStore } from "@/stores/auth";

import GlobalSearch from "./GlobalSearch.vue";
import NotificationsMenu from "./NotificationsMenu.vue";
import UserMenu from "./UserMenu.vue";

/**
 * The application header, ported from web/template/home.gohtml. Classes are copied so
 * a migrated page and a server-rendered one line up while both exist.
 */
withDefaults(
  defineProps<{
    /**
     * Shows the mobile navigation toggle. Off by default: the sidebar it opens belongs
     * to the unmigrated start page.
     */
    showSidenavToggle?: boolean;
    /** Renders only the logo, where the header's controls do not apply. */
    minimal?: boolean;
  }>(),
  { showSidenavToggle: false, minimal: false },
);

const emit = defineEmits<{ "toggle-sidenav": [] }>();

const auth = useAuthStore();

// Served by Go from the branding directory, so it must not be bundled as an asset.
const LOGO_URL = "/logo.svg";
</script>

<template>
  <header
    class="text-3 z-50 flex h-16 w-full shrink-0 grow-0 items-center justify-between px-3 py-2"
  >
    <div class="flex items-center">
      <button
        v-if="showSidenavToggle && !minimal"
        id="open-sidenav"
        type="button"
        title="Open Sidenav"
        class="tum-live-icon-button p-3 text-lg md:hidden"
        @click="emit('toggle-sidenav')"
      >
        <i class="fa-solid fa-bars"></i>
      </button>
      <!-- A full navigation: "/" is still served by a template. -->
      <a href="/" class="mx-3" id="logo" title="Start">
        <img :src="LOGO_URL" :width="minimal ? 42 : 64" alt="TUM-Live Logo" />
      </a>
    </div>

    <template v-if="!minimal">
      <GlobalSearch />

      <div id="user-context" class="ms-auto flex items-center">
        <template v-if="auth.user">
          <NotificationsMenu />
          <UserMenu />
        </template>
        <a
          v-else-if="auth.loaded"
          href="/login"
          class="tum-live-button tum-live-button-primary mx-3"
        >
          Login
        </a>
      </div>
    </template>
  </header>
</template>
