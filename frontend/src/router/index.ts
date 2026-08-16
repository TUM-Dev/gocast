import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";

import LoginView from "@/views/LoginView.vue";
import SettingsView from "@/views/SettingsView.vue";

declare module "vue-router" {
  interface RouteMeta {
    /** Renders the logo-only header instead of the full application header. */
    minimalHeader?: boolean;
    /** Renders the site footer below the page. */
    footer?: boolean;
    /** Reached before signing in; the app skips loading the user, which would fail. */
    anonymous?: boolean;
  }
}

/**
 * Client routes. Every path must also be in web/router.go's spaRoutes, or Go answers
 * with the legacy template and this router never sees the request.
 */
const routes: RouteRecordRaw[] = [
  {
    path: "/settings",
    name: "settings",
    component: SettingsView,
  },
  {
    path: "/login",
    name: "login",
    component: LoginView,
    // Search, notifications and the account menu do not apply before signing in. The
    // server-rendered login page uses the same reduced chrome.
    meta: { minimalHeader: true, footer: true, anonymous: true },
  },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});

/** Unclaimed paths belong to pages Go still renders, so hand them back. */
router.beforeEach((to, _from, next) => {
  if (to.matched.length === 0) {
    window.location.assign(to.fullPath);
    return;
  }
  next();
});
