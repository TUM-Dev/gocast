<script setup lang="ts">
import { computed, watch } from "vue";
import { useRoute } from "vue-router";

import { groupBySemester } from "@/lib/course-admin";
import { sameSemester } from "@/lib/semesters";
import { can, type Permission } from "@/lib/settings";
import { useAuthStore } from "@/stores/auth";
import { useCourseAdminStore } from "@/stores/course-admin";

/**
 * The frame every administration page sits in.
 *
 * Each entry names the permission its route enforces, so a link is offered exactly
 * when following it works; the template gated the whole block on `Role == 1`, which
 * stopped matching the server once those routes split across two permissions.
 *
 * Below the course links sits the tree of every course the user administers, by
 * semester, as the template's sidebar had it. Only someone with the lecture
 * permission is asked for it: listAdministeredCourses refuses anyone else.
 */
interface AdminLink {
  label: string;
  path: string;
  /** What web/router.go requires for this path. */
  permission: Permission;
  /** Whether the SPA owns this page; the rest are plain links to Go. */
  migrated?: boolean;
}

const administration: AdminLink[] = [
  { label: "Users", path: "/admin/users", permission: "users.manage", migrated: true },
  {
    label: "Lecture Halls",
    path: "/admin/lecture-halls",
    permission: "server.administer",
    migrated: true,
  },
  { label: "Workers", path: "/admin/workers", permission: "server.administer", migrated: true },
  { label: "Runners", path: "/admin/runners", permission: "server.administer", migrated: true },
  {
    label: "Server Notifications",
    path: "/admin/server-notifications",
    permission: "server.administer",
    migrated: true,
  },
  {
    label: "User Notifications",
    path: "/admin/notifications",
    permission: "server.administer",
    migrated: true,
  },
  {
    label: "Server Statistics",
    path: "/admin/server-stats",
    permission: "server.administer",
    migrated: true,
  },
  {
    label: "Course Import",
    path: "/admin/course-import",
    permission: "server.administer",
    migrated: true,
  },
  { label: "Token Management", path: "/admin/token", permission: "users.manage", migrated: true },
  { label: "Integrations", path: "/admin/integrations", permission: "server.administer", migrated: true },
  { label: "Audits", path: "/admin/audits", permission: "server.administer", migrated: true },
  {
    label: "Info Pages",
    path: "/admin/info-pages",
    permission: "server.administer",
    migrated: true,
  },
  {
    label: "Maintenance",
    path: "/admin/maintenance",
    permission: "server.administer",
    migrated: true,
  },
];

const courses: AdminLink[] = [
  { label: "Schedule", path: "/admin", permission: "lecture", migrated: true },
  { label: "Create Course", path: "/admin/create-course", permission: "lecture", migrated: true },
];

const auth = useAuthStore();

const allowed = (links: AdminLink[]) =>
  computed(() => links.filter((link) => can(auth.user, link.permission)));

const administrationLinks = allowed(administration);
const courseLinks = allowed(courses);

const courseAdmin = useCourseAdminStore();
watch(
  () => can(auth.user, "lecture"),
  (lecturer) => {
    if (lecturer) void courseAdmin.loadAdministered();
  },
  { immediate: true },
);

// Every tab of a course's page highlights it, not only the one its link points at.
const route = useRoute();
const openCourseId = computed(() => Number(route.params.courseID) || 0);

const semesterGroups = computed(() =>
  can(auth.user, "lecture") ? groupBySemester(courseAdmin.administered) : [],
);
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
        <template v-for="link in courseLinks" :key="link.path">
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

        <!-- Native <details>: collapsed semesters stay out of the tab order and out
             of what assistive technology reads, with nothing to wire up. -->
        <details
          v-for="group in semesterGroups"
          :key="group.label"
          :open="sameSemester(group.semester, courseAdmin.currentSemester ?? undefined)"
          class="tum-live-side-navigation-group-item"
        >
          <summary class="text-4 cursor-pointer text-sm">{{ group.label }}</summary>
          <!-- The settings tab for now; the lectures tab is still a server page. -->
          <RouterLink
            v-for="course in group.courses"
            :key="course.id"
            :to="`/admin/courses/${course.id}/settings`"
            class="hover block truncate py-1 pl-3 text-sm"
            :title="course.name"
          >
            <span :class="course.id === openCourseId ? 'text-1 font-semibold' : 'text-5'">{{
              course.name
            }}</span>
          </RouterLink>
        </details>
      </section>
    </nav>

    <article class="text-3 min-w-0 grow p-4">
      <slot />
    </article>
  </div>
</template>
