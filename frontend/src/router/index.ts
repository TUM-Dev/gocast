import {
  START_LOCATION,
  createRouter,
  createWebHistory,
  type RouteLocationNormalized,
  type RouteLocationRaw,
  type RouteRecordRaw,
} from "vue-router";

import { isChunkLoadError, reloadOnce } from "@/lib/chunk-reload";
import { singleQueryParam } from "@/lib/route-query";
import HomeView from "@/views/HomeView.vue";
import LoginView from "@/views/LoginView.vue";

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
    /** Renders no header: the page is a window of its own, such as the popup chat. */
    bare?: boolean;
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
    component: () => import("@/views/MyCoursesView.vue"),
  },
  {
    path: "/courses/public",
    name: "public-courses",
    meta: { sidenav: true, footer: true },
    component: () => import("@/views/PublicCoursesView.vue"),
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
    component: () => import("@/views/CourseView.vue"),
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
    component: () => import("@/views/SettingsView.vue"),
  },
  {
    // Visitors search public courses, a signed-in user finds more; the shell loads
    // the user as it does for the start page.
    path: "/search",
    name: "search",
    component: () => import("@/views/SearchView.vue"),
    meta: { footer: true },
  },
  {
    // The lectures saved on this device. Anonymous because it must open offline, where
    // loading the user can only fail; everything it shows comes from the device.
    path: "/offline",
    name: "offline",
    component: () => import("@/views/OfflineView.vue"),
    meta: { footer: true, anonymous: true },
  },
  // The three built-in pages keep their own routes rather than falling under
  // "/:slug" below, so their URLs are unaffected by whatever an administrator does
  // to the dynamic ones. Anonymous because nothing on these pages is per-user.
  ...(["privacy", "imprint", "about"] as const).map(
    (name): RouteRecordRaw => ({
      path: `/${name}`,
      name,
      component: () => import("@/views/InfoPageView.vue"),
      props: { name },
      meta: { anonymous: true, footer: true },
    }),
  ),
  {
    // Any further info page an administrator has added. Static routes above and in
    // this array outrank a dynamic segment, so this only ever catches a path nothing
    // more specific claims; see InfoPageDynamicView for what happens next.
    path: "/:slug",
    name: "info-page",
    component: () => import("@/views/InfoPageDynamicView.vue"),
    meta: { anonymous: true, footer: true },
  },
  {
    // The administration pages, migrating one at a time. Each is registered in
    // web/router.go inside the permission group that guards it, so an unauthorized
    // caller is refused the shell rather than reaching an empty page.
    path: "/admin/integrations",
    name: "admin-integrations",
    component: () => import("@/views/admin/IntegrationsView.vue"),
  },
  {
    path: "/admin/runners",
    name: "admin-runners",
    component: () => import("@/views/admin/RunnersView.vue"),
  },
  {
    path: "/admin/users",
    name: "admin-users",
    component: () => import("@/views/admin/UsersView.vue"),
  },
  {
    path: "/admin/info-pages",
    name: "admin-info-pages",
    component: () => import("@/views/admin/InfoPagesView.vue"),
  },
  {
    path: "/admin/workers",
    name: "admin-workers",
    component: () => import("@/views/admin/WorkersView.vue"),
  },
  {
    path: "/admin/maintenance",
    name: "admin-maintenance",
    component: () => import("@/views/admin/MaintenanceView.vue"),
  },
  {
    path: "/admin/course-import",
    name: "admin-course-import",
    component: () => import("@/views/admin/CourseImportView.vue"),
  },
  {
    path: "/admin/token",
    name: "admin-token",
    component: () => import("@/views/admin/TokensView.vue"),
  },
  {
    path: "/admin/server-notifications",
    name: "admin-server-notifications",
    component: () => import("@/views/admin/ServerNotificationsView.vue"),
  },
  {
    path: "/admin/notifications",
    name: "admin-notifications",
    component: () => import("@/views/admin/NotificationsView.vue"),
  },
  {
    path: "/admin/audits",
    name: "admin-audits",
    component: () => import("@/views/admin/AuditsView.vue"),
  },
  {
    path: "/admin/server-stats",
    name: "admin-server-stats",
    component: () => import("@/views/admin/ServerStatsView.vue"),
  },
  {
    path: "/admin/lecture-halls",
    name: "admin-lecture-halls",
    component: () => import("@/views/admin/LectureHallsView.vue"),
  },
  {
    path: "/admin/lecture-halls/new",
    name: "admin-lecture-halls-new",
    component: () => import("@/views/admin/LectureHallCreateView.vue"),
  },
  {
    // A course's administration page; each tab is a child route (rule 4 in
    // web/router.go). The parent path is not in spaRoutes, so Go does not serve it;
    // the redirect only catches an in-app navigation to it.
    path: "/admin/courses/:courseID",
    component: () => import("@/components/admin/CourseAdminLayout.vue"),
    children: [
      { path: "", redirect: { name: "admin-course-lectures" } },
      { path: "lectures", name: "admin-course-lectures", component: () => import("@/views/admin/CourseLecturesView.vue") },
      { path: "settings", name: "admin-course-settings", component: () => import("@/views/admin/CourseSettingsView.vue") },
      { path: "stats", name: "admin-course-stats", component: () => import("@/views/admin/CourseStatsView.vue") },
      { path: "participants", name: "admin-course-participants", component: () => import("@/views/admin/CourseParticipantsView.vue") },
    ],
  },
  {
    path: "/admin/courses/:courseID/lectures/:streamID/stats",
    name: "admin-lecture-stats",
    component: () => import("@/views/admin/LectureStatsView.vue"),
  },
  {
    // The lecturer's view of a running lecture; replaces /admin/management/:c/:s. Not in
    // spaRoutes until the watch flip, which also redirects the old URL here.
    path: "/admin/courses/:courseID/lectures/:streamID/live",
    name: "admin-lecture-live",
    component: () => import("@/views/admin/LectureLiveView.vue"),
  },
  {
    path: "/admin/create-course",
    name: "admin-create-course",
    component: () => import("@/views/admin/CreateCourseView.vue"),
  },
  {
    path: "/admin",
    name: "admin-schedule",
    component: () => import("@/views/admin/ScheduleView.vue"),
  },
  {
    path: "/login",
    name: "login",
    component: LoginView,
    // Search, notifications and the account menu do not apply before signing in. The
    // server-rendered login page uses the same reduced chrome.
    meta: { minimalHeader: true, footer: true, anonymous: true },
  },
  {
    // The link a password reset or account invite mails. camelCase, unlike every other
    // path: it is in emails that were already sent. The param isn't called `key`:
    // Vue reserves that name, so the view would never receive it as a prop.
    path: "/setPassword/:resetKey",
    name: "set-password",
    component: () => import("@/views/SetPasswordView.vue"),
    props: true,
    meta: { minimalHeader: true, footer: true, anonymous: true },
  },
  {
    // The links mailed to a lecturer when their course is imported; the token in the
    // query string is the credential. Two paths, one view.
    path: "/edit-course",
    name: "course-opt-in",
    component: () => import("@/views/CourseTokenView.vue"),
    props: { mode: "opt-in" },
    meta: { minimalHeader: true, footer: true, anonymous: true },
  },
  {
    path: "/edit-course/opt-out",
    name: "course-opt-out",
    component: () => import("@/views/CourseTokenView.vue"),
    props: { mode: "opt-out" },
    meta: { minimalHeader: true, footer: true, anonymous: true },
  },
  {
    // The watch page. `:version` is the layout (COMB, PRES, CAM or SPLIT). Anonymous
    // because a public course's stream is watchable without a session. The version is checked by the view.
    path: "/w/:slug/:streamID",
    name: "watch",
    component: () => import("@/views/WatchView.vue"),
    meta: { anonymous: true },
  },
  {
    path: "/w/:slug/:streamID/:version",
    name: "watch-layout",
    component: () => import("@/views/WatchView.vue"),
    meta: { anonymous: true },
  },
  {
    // The chat on a page of its own, for a window beside the player. Two URLs, one
    // page: `/chat/popup` is what the old player's popup button and any old link use.
    // Anonymous because the chat is readable without a session on a public stream.
    path: "/w/:slug/:streamID/chat/popup",
    name: "stream-chat-popup",
    component: () => import("@/views/ChatPopupView.vue"),
    meta: { bare: true, anonymous: true },
  },
  {
    path: "/w/:slug/:streamID/chat",
    name: "stream-chat",
    component: () => import("@/views/ChatPopupView.vue"),
    meta: { bare: true, anonymous: true },
  },
  {
    // An application asking to connect to a course. Go keeps the sign-in redirect and
    // serves this page unframeable; the state in the query is the application's.
    path: "/integration/authorize/:id",
    name: "integration-authorize",
    component: () => import("@/views/IntegrationAuthorizationView.vue"),
    meta: { minimalHeader: true, footer: true },
  },
  {
    // Where "/" sends a fresh deployment; the page refuses once an account exists.
    path: "/onboarding",
    name: "onboarding",
    component: () => import("@/views/OnboardingView.vue"),
    meta: { minimalHeader: true, footer: true, anonymous: true },
  },
  {
    // Everything nothing above claims. On a first load that is Go's 404, which serves
    // the shell with that status; an in-app navigation here is handed back to Go in the
    // guard below.
    path: "/:pathMatch(.*)*",
    name: "not-found",
    component: () => import("@/views/NotFoundView.vue"),
    meta: { anonymous: true, footer: true },
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

/**
 * Whether the navigation in flight is the page load itself. A path the SPA does not
 * own is handed back to Go on an in-app navigation, but on a first load Go has
 * already answered -- with the shell and a 404 -- and handing it back would reload
 * forever.
 */
let inAppNavigations = 0;
export const isInitialNavigation = (): boolean => inAppNavigations === 0;

router.beforeEach((to, from, next) => {
  // Counted as soon as it starts, so a view mounted by the first load still sees the
  // load as initial whichever order the router's hooks and the mount run in.
  if (from !== START_LOCATION) inAppNavigations += 1;

  const legacy = legacyStartPageRedirect(to);
  if (legacy) {
    next(legacy);
    return;
  }

  /**
   * Unclaimed paths in an in-app navigation may be ones only Go answers (/logout, the
   * redirects of renamed URLs, /healthcheck), so hand them back; Go's own 404 for the
   * rest arrives as a first load.
   */
  if (to.name === "not-found" && !isInitialNavigation()) {
    window.location.assign(to.fullPath);
    return;
  }

  next();
});

// A stale tab after a deploy asks for a component file that no longer exists. The
// navigation is dropped by the router, so load the page it was headed for afresh.
router.onError((err, to) => {
  if (isChunkLoadError(err)) reloadOnce(to.fullPath);
});
