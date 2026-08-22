<script setup lang="ts">
import { onMounted, watch } from "vue";
import { useRoute } from "vue-router";

import { useSemesterQuery } from "@/lib/route-query";
import { useCourseStore } from "@/stores/courses";
import { useSemesterStore } from "@/stores/semester";
import { useSidenavStore } from "@/stores/sidenav";

import StartPageSidenav from "./StartPageSidenav.vue";

/**
 * The frame shared by the four views of the start page: the sidebar, and the column
 * beside it that each view fills. Ported from the `#content` section of
 * web/template/home.gohtml.
 *
 * Loading the listings is the layout's job rather than each view's: the sidebar shows
 * the same three listings the views do, and the store holds one copy for both.
 */
const courses = useCourseStore();
const semesters = useSemesterStore();
const sidenav = useSidenavStore();
const route = useRoute();
const { year, term } = useSemesterQuery();

async function load(): Promise<void> {
  // The listings are per semester, and which semester "no semester in the URL" means
  // is the server's answer — so wait for it rather than fetching twice. A failure is
  // survivable: `resolve` falls back to undefined, which the API reads as the current
  // semester, and the page still fills. Blocking on it left the whole start page empty.
  await semesters.load().catch(() => {});
  await courses.load(semesters.resolve(year.value, term.value));
}

onMounted(() => {
  load().catch(() => {});
});

// A semester change is a new set of listings; the views re-render from the store.
watch([year, term], () => {
  load().catch(() => {});
});

// The sidebar covers the whole screen on a narrow one, so leaving it open across a
// navigation would hide the page just navigated to.
watch(() => route.fullPath, sidenav.close);
</script>

<template>
  <div class="flex w-full grow">
    <StartPageSidenav />

    <!-- Hidden rather than pushed aside while the sidebar is covering the screen. -->
    <article class="text-3 grow p-4" :class="{ hidden: sidenav.open }">
      <slot />
    </article>
  </div>
</template>
