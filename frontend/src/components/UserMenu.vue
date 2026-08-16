<script setup lang="ts">
import { ref } from "vue";

import { useClickOutside } from "@/lib/useClickOutside";
import { useAuthStore } from "@/stores/auth";
import { useThemeStore, type ThemeMode } from "@/stores/theme";

import KeyboardShortcuts from "./KeyboardShortcuts.vue";

const auth = useAuthStore();
const theme = useThemeStore();

const open = ref(false);
const themePickerOpen = ref(false);
const shortcutsOpen = ref(false);
const root = ref<HTMLElement | null>(null);

// model.AdminType and model.LecturerType (model/user.go).
const ADMIN_ROLE = 1;
const LECTURER_ROLE = 2;

useClickOutside(root, () => {
  open.value = false;
});

function isStaff(role: number): boolean {
  return role === ADMIN_ROLE || role === LECTURER_ROLE;
}

function setTheme(mode: ThemeMode): void {
  theme.setMode(mode);
}
</script>

<template>
  <div ref="root">
    <button
      type="button"
      class="tum-live-border-primary mx-3 flex h-[32px] w-[32px] items-center justify-center rounded-full border-2"
      :title="auth.user ? `Signed in as ${auth.user.name}` : 'Account'"
      @click="open = !open"
    >
      <i class="fa-solid fa-user tum-live-font-primary"></i>
    </button>

    <div class="relative">
      <article
        v-show="open"
        class="tum-live-menu absolute right-[50%] top-full mt-2 h-fit w-56 overflow-hidden"
      >
        <header>
          <p class="font-semibold">Signed in as</p>
          <span>@{{ auth.user?.name }}</span>
        </header>

        <nav class="d-grid gap-3 font-light">
          <template v-if="auth.user && isStaff(auth.user.role)">
            <a href="/admin" class="tum-live-menu-item">
              <div class="icon-wrapper mr-2"><i class="fa-solid fa-hammer"></i></div>
              <p>Admin</p>
            </a>
            <div class="border-b dark:border-gray-800"></div>
          </template>

          <div>
            <button
              type="button"
              class="tum-live-menu-item"
              @click="themePickerOpen = !themePickerOpen"
            >
              <div class="icon-wrapper mr-2"><i class="fa-regular fa-moon"></i></div>
              <span>Theme</span>
              <i
                class="fa-solid ml-auto"
                :class="themePickerOpen ? 'fa-chevron-up' : 'fa-chevron-down'"
              ></i>
            </button>
            <div v-show="themePickerOpen" class="grid gap-1">
              <button
                v-for="mode in theme.modes"
                :key="mode.id"
                type="button"
                class="px-10 py-1 text-left hover:bg-gray-100 dark:hover:bg-gray-800"
                :class="{ 'bg-gray-100 dark:bg-gray-800': mode.id === theme.mode }"
                @click="setTheme(mode.id)"
              >
                {{ mode.name }}
              </button>
            </div>
          </div>

          <!--
            A router-link: /settings is served by the SPA, so this is a client-side
            navigation. The other entries point at pages Go still renders.
          -->
          <RouterLink to="/settings" class="tum-live-menu-item" @click="open = false">
            <div class="icon-wrapper mr-2"><i class="fa-solid fa-gear"></i></div>
            <p>Settings</p>
          </RouterLink>

          <button type="button" class="tum-live-menu-item" @click="shortcutsOpen = true">
            <div class="icon-wrapper mr-2"><i class="fa-solid fa-keyboard"></i></div>
            <p>Keyboard Shortcuts</p>
          </button>

          <div class="border-b dark:border-gray-800"></div>

          <a
            href="https://github.com/TUM-Dev/gocast"
            target="_blank"
            rel="noopener"
            class="tum-live-menu-item"
          >
            <div class="icon-wrapper mr-2"><i class="fa-regular fa-comment"></i></div>
            <p>Send Feedback</p>
          </a>
          <a
            href="https://github.com/TUM-Dev/gocast/issues/new?assignees=&labels=&template=bug_report.md&title="
            target="_blank"
            rel="noopener"
            class="tum-live-menu-item"
          >
            <div class="icon-wrapper mr-2"><i class="fa-brands fa-github"></i></div>
            <p>Report problem</p>
          </a>

          <div class="border-b dark:border-gray-800"></div>

          <a href="/logout" class="tum-live-menu-item">
            <div class="icon-wrapper mr-2"><i class="fa-solid fa-sign-out"></i></div>
            <p>Logout</p>
          </a>
        </nav>
      </article>
    </div>

    <KeyboardShortcuts :open="shortcutsOpen" @close="shortcutsOpen = false" />
  </div>
</template>
