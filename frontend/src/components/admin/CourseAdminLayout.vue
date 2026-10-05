<script setup lang="ts">
import { computed, watch } from "vue";
import { useRoute } from "vue-router";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { publicCoursePath } from "@/lib/course-admin";
import { semesterLabel } from "@/lib/semesters";
import { redirectToLogin, useAuthStore } from "@/stores/auth";
import { useCourseAdminStore } from "@/stores/course-admin";

/**
 * A course's administration page: the course's name and a tab per area, each tab a
 * child route (rule 4 in web/router.go). The course is loaded once here and shared
 * with the tabs through the course-admin store.
 *
 * The server has already refused anyone who does not administer the course before
 * serving this page; getCourseAdmin refuses them again, so the page cannot be the
 * only guard.
 */
const auth = useAuthStore();
const route = useRoute();
const store = useCourseAdminStore();

const courseId = computed(() => Number(route.params.courseID));

interface Tab {
  label: string;
  /** A child route of this page, or `href` for a page Go still renders. */
  to?: string;
  href?: string;
}

const tabs = computed<Tab[]>(() => [
  { label: "Lectures", to: `/admin/courses/${courseId.value}/lectures` },
  { label: "Settings", to: `/admin/courses/${courseId.value}/settings` },
  { label: "Statistics", to: `/admin/courses/${courseId.value}/stats` },
  { label: "Participants", to: `/admin/courses/${courseId.value}/participants` },
]);

watch(
  courseId,
  async (id) => {
    const user = await auth.load().catch(() => null);
    if (!user) {
      redirectToLogin();
      return;
    }
    await store.load(id);
  },
  { immediate: true },
);

/** Set by the copy action, to the number of lectures and admins that did not come along. */
const copiedNotice = computed(() => {
  const copied = route.query.copied;
  if (typeof copied !== "string") return "";
  const failed = Number(copied) || 0;
  return failed > 0
    ? `The course was copied, but ${failed} of its lectures or administrators could not be. ` +
        "Check the lectures tab."
    : "The course was copied. These are the settings of the new course.";
});

const tabClass = (active: boolean) =>
  [
    "inline-block rounded-t-lg border-b-2 px-4 py-3",
    active
      ? "text-1 border-blue-600 font-semibold dark:border-blue-400"
      : "text-4 border-transparent hover:border-gray-300 dark:hover:border-gray-600",
  ].join(" ");
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex w-full max-w-5xl flex-col gap-4">
      <p v-if="store.error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
        {{ store.error }}
      </p>
      <p v-else-if="!store.course" class="text-5 text-sm">Loading course…</p>

      <template v-if="store.course && store.course.id === courseId">
        <header class="flex flex-col gap-1">
          <h1 class="text-1 text-2xl font-bold break-words">
            <a :href="publicCoursePath(store.course)" class="hover:underline">{{ store.course.name }}</a>
          </h1>
          <p class="text-5 text-sm">
            {{ semesterLabel({ year: store.course.year, term: store.course.term }) }}
          </p>
        </header>

        <p
          v-if="copiedNotice"
          class="rounded-lg border border-green-300 bg-green-50 px-3 py-2 text-sm text-green-800 dark:border-green-900 dark:bg-green-950 dark:text-green-300"
          role="status"
        >
          {{ copiedNotice }}
        </p>

        <nav aria-label="Course administration" class="border-b border-gray-200 dark:border-gray-700">
          <ul class="-mb-px flex flex-wrap text-sm">
            <li v-for="tab in tabs" :key="tab.label">
              <RouterLink v-if="tab.to" v-slot="{ href, navigate, isExactActive }" :to="tab.to" custom>
                <a
                  :href="href"
                  :class="tabClass(isExactActive)"
                  :aria-current="isExactActive ? 'page' : undefined"
                  @click="navigate"
                  >{{ tab.label }}</a
                >
              </RouterLink>
              <a v-else :href="tab.href" :class="tabClass(false)">{{ tab.label }}</a>
            </li>
          </ul>
        </nav>

        <RouterView />
      </template>
    </section>
  </AdminLayout>
</template>
