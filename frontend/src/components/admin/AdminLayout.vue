<script setup lang="ts">
import { computed } from "vue";

import { can, type Permission } from "@/lib/settings";
import { useAuthStore } from "@/stores/auth";

/**
 * The frame every administration page sits in, ported from the sidebar in
 * web/template/admin/admin.gohtml.
 *
 * Two things differ from the template deliberately.
 *
 * The template gated the whole Administration block on `Role == 1`, which stopped
 * agreeing with the server the moment web/router.go split those routes across
 * PermAdministerServer and PermManageUsers. Here each entry names the permission its
 * route actually enforces, so a link is offered exactly when following it works.
 *
 * And the sidebar's tree of administered courses per semester is not here: it needs a
 * "courses I administer" endpoint that v2 does not have yet. Until it does, the
 * Courses group links to the server-rendered schedule, which still has the tree. See
 * frontend/README.md.
 */
interface AdminLink {
  label: string;
  path: string;
  /** What web/router.go requires for this path. */
  permission: Permission;
  /**
   * Whether the SPA owns this page. The rest are still rendered by Go, so they are
   * plain links: a RouterLink to one would match nothing and bounce through the
   * server anyway.
   */
  migrated?: boolean;
}

const administration: AdminLink[] = [
  { label: "Users", path: "/admin/users", permission: "users.manage" },
  { label: "Lecture Halls", path: "/admin/lecture-halls", permission: "server.administer" },
  { label: "Workers", path: "/admin/workers", permission: "server.administer" },
  { label: "Runners", path: "/admin/runners", permission: "server.administer", migrated: true },
  {
    label: "Server Notifications",
    path: "/admin/server-notifications",
    permission: "server.administer",
  },
  { label: "User Notifications", path: "/admin/notifications", permission: "server.administer" },
  { label: "Server Statistics", path: "/admin/server-stats", permission: "server.administer" },
  { label: "Course Import", path: "/admin/course-import", permission: "server.administer" },
  { label: "Token Management", path: "/admin/token", permission: "users.manage" },
  { label: "Audits", path: "/admin/audits", permission: "server.administer" },
  { label: "Info Pages", path: "/admin/info-pages", permission: "server.administer" },
  { label: "Maintenance", path: "/admin/maintenance", permission: "server.administer" },
];

const courses: AdminLink[] = [
  { label: "Schedule", path: "/admin", permission: "lecture" },
  { label: "Create Course", path: "/admin/create-course", permission: "lecture" },
];

const auth = useAuthStore();

const allowed = (links: AdminLink[]) =>
  computed(() => links.filter((link) => can(auth.user, link.permission)));

const administrationLinks = allowed(administration);
const courseLinks = allowed(courses);
</script>

<template>
  <div class="flex w-full grow">
    <nav class="tum-live-side-navigation md:block md:w-56 lg:w-72" aria-label="Administration">
      <section v-if="administrationLinks.length" class="tum-live-side-navigation-group">
        <header class="text-2 text-xs uppercase tracking-wide">Administration</header>
        <template v-for="link in administrationLinks" :key="link.path">
          <RouterLink
            v-if="link.migrated"
            v-slot="{ isActive }"
            :to="link.path"
            class="tum-live-side-navigation-group-item hover block"
          >
            <span :class="isActive ? 'text-1 font-semibold' : 'text-5'">{{ link.label }}</span>
          </RouterLink>
          <a
            v-else
            :href="link.path"
            class="tum-live-side-navigation-group-item hover text-5 block"
            >{{ link.label }}</a
          >
        </template>
      </section>

      <section v-if="courseLinks.length" class="tum-live-side-navigation-group">
        <header class="text-2 text-xs uppercase tracking-wide">Courses</header>
        <a
          v-for="link in courseLinks"
          :key="link.path"
          :href="link.path"
          class="tum-live-side-navigation-group-item hover text-5 block"
          >{{ link.label }}</a
        >
      </section>
    </nav>

    <article class="text-3 grow p-4">
      <slot />
    </article>
  </div>
</template>
