import {
  createRouter,
  createWebHistory,
  type RouteLocationNormalized,
  type RouteLocationRaw,
  type RouteRecordRaw,
} from "vue-router";

import { singleQueryParam } from "@/lib/route-query";
import CourseView from "@/views/CourseView.vue";
import HomeView from "@/views/HomeView.vue";
import LoginView from "@/views/LoginView.vue";
import RunnersView from "@/views/admin/RunnersView.vue";
import MyCoursesView from "@/views/MyCoursesView.vue";
import PublicCoursesView from "@/views/PublicCoursesView.vue";
import SettingsView from "@/views/SettingsView.vue";

declare module "vue-router" {
  interface RouteMeta {
    /** Renders the logo-only header instead of the full application header. */
    minimalHeader?: boolean;
    /** Renders the site footer below the page. */
    footer?: boolean;
    /** Reached before signing in; the app skips loading the user, which would fail. */
    anonymous?: boolean;
    /**
     * The page has the start page's sidebar, so the header offers the button that
     * opens it on a narrow screen.
     */
    sidenav?: boolean;
  }
}

/**
 * Which semester a page is about is a query parameter — `?year=2026&term=W` — on every
 * route but the course page, which keeps the path shape the server already redirects
 * to. Omitting it means the current semester, which only the API knows.
 */
const routes: RouteRecordRaw[] = [
  {
    path: "/",
    name: "home",
    meta: { sidenav: true, footer: true },
    component: HomeView,
  },
  {
    path: "/courses/mine",
    name: "my-courses",
    meta: { sidenav: true, footer: true },
    component: MyCoursesView,
  },
  {
    path: "/courses/public",
    name: "public-courses",
    meta: { sidenav: true, footer: true },
    component: PublicCoursesView,
  },
  {
    // The shape `/course/:year/:term/:slug` in web/router.go already redirects to, so
    // links to it keep working unchanged. Year and term stay in the path here rather
    // than moving to the query, because that is the URL people have bookmarked.
    //
    // There is deliberately no `/course/:slug` companion for the current semester:
    // gin panics when two parameters with different names share a position, so it
    // could never be registered server-side beside this one.
    path: "/course/:year/:term/:slug",
    name: "course",
    meta: { sidenav: true, footer: true },
    component: CourseView,
    props: true,
  },
  {
    // Server-side this is a redirect into the query form. Repeated here so an
    // in-app navigation to it resolves without a round trip.
    path: "/semester/:year/:term",
    redirect: (to) => ({ name: "home", query: { year: to.params.year, term: to.params.term } }),
  },
  {
    path: "/settings",
    name: "settings",
    component: SettingsView,
  },
  {
    // The administration pages, migrating one at a time. Each is registered in
    // web/router.go inside the permission group that guards it, so an unauthorized
    // caller is refused the shell rather than reaching an empty page.
    path: "/admin/runners",
    name: "admin-runners",
    component: RunnersView,
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

/**
 * The four views of the server-rendered start page, which were one path and a `view`
 * query parameter rather than four routes. The numbers are the View enum in
 * web/ts/views/home.ts, and they are in links, in browser history and in bookmarks.
 */
const enum LegacyView {
  Main = "0",
  UserCourses = "1",
  PublicCourses = "2",
  Course = "3",
}

/**
 * Translates a URL of the server-rendered start page into the route that replaced it,
 * or returns null when `to` is not one.
 *
 * Exported for the tests: this is the only thing standing between an old bookmark and
 * an empty page, and nothing else would notice if it stopped working.
 */
export function legacyStartPageRedirect(to: RouteLocationNormalized): RouteLocationRaw | null {
  if (to.path !== "/") return null;

  const year = singleQueryParam(to.query, "year");
  const term = singleQueryParam(to.query, "term");
  const slug = singleQueryParam(to.query, "slug");
  const view = singleQueryParam(to.query, "view") ?? LegacyView.Main;

  // The semester survives every translation; on the main view it is already in the
  // form this router uses, so that URL is left alone rather than redirected to itself.
  const semester = year && term ? { year, term } : {};

  switch (view) {
    case LegacyView.UserCourses:
      return { name: "my-courses", query: semester };
    case LegacyView.PublicCourses:
      return { name: "public-courses", query: semester };
    case LegacyView.Course:
      // The course route has nowhere to put a slug without a semester. Nothing
      // generated such a URL — the old page wrote year and term into history on every
      // navigation — so this falls back to the start page rather than growing a
      // second course route that gin could not register beside the first.
      if (!slug || !year || !term) return { name: "home" };
      return { name: "course", params: { year, term, slug } };
    default:
      // `?view=0` and a stray `?slug=` are the main view with noise attached; drop it
      // so the address bar reads the same as it would after navigating there.
      if (to.query.view !== undefined || to.query.slug !== undefined) {
        return { name: "home", query: semester };
      }
      return null;
  }
}

export const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach((to, _from, next) => {
  const legacy = legacyStartPageRedirect(to);
  if (legacy) {
    next(legacy);
    return;
  }

  /** Unclaimed paths belong to pages Go still renders, so hand them back. */
  if (to.matched.length === 0) {
    window.location.assign(to.fullPath);
    return;
  }

  next();
});
