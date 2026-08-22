<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";

import { courseUrl, type Course } from "@/lib/courses";
import { useSemesterQuery } from "@/lib/route-query";
import { sameSemester, semesterLabel, type Semester } from "@/lib/semesters";
import { useCourseStore } from "@/stores/courses";
import { useSemesterStore } from "@/stores/semester";
import { useSidenavStore } from "@/stores/sidenav";

import AppFooter from "./AppFooter.vue";

/**
 * The start page's sidebar, ported from web/template/home.gohtml. Classes are copied
 * so it lines up with the server-rendered page while both exist.
 *
 * How many courses each group shows before offering its full listing is carried over
 * as well: eight of the user's own, five public.
 */
const USER_COURSES_SHOWN = 8;
const PUBLIC_COURSES_SHOWN = 5;

const router = useRouter();
const courses = useCourseStore();
const semesters = useSemesterStore();
const sidenav = useSidenavStore();
const { year, term } = useSemesterQuery();

const showAllSemesters = ref(false);

onMounted(() => {
  semesters.load().catch(() => {});
});

const selected = computed(() => semesters.resolve(year.value, term.value));

/**
 * Switching semester always lands on the start page, never on the current one.
 * A course belongs to the semester in its own URL, so carrying a slug across would
 * ask for a course that does not exist in the semester just chosen.
 */
function switchSemester(semester: Semester): void {
  showAllSemesters.value = false;
  sidenav.close();
  router.push({ name: "home", query: { year: semester.year, term: semester.term } });
}

function openCourse(course: Course): void {
  sidenav.close();
  router.push(courseUrl(course));
}
</script>

<template>
  <section
    id="side-navigation"
    class="tum-live-side-navigation md:block md:w-56 lg:w-80"
    :class="sidenav.open ? 'flex w-full' : 'hidden'"
  >
    <article class="tum-live-side-navigation-group">
      <header>
        <i class="fa-solid fa-calendar"></i>
        Semesters
      </header>

      <span
        v-if="!showAllSemesters"
        class="tum-live-side-navigation-group-item mb-4 border-l-4 border-blue-500/50 bg-blue-100/50 dark:border-indigo-600/50 dark:bg-indigo-500/25"
      >
        {{ selected ? semesterLabel(selected) : "…" }}
      </span>

      <template v-else>
        <button
          v-for="semester in semesters.semesters"
          :key="`${semester.year}${semester.term}`"
          type="button"
          class="tum-live-side-navigation-group-item hover"
          :class="{
            'border-l-4 border-blue-500/50 bg-blue-100/50 dark:border-indigo-600/50 dark:bg-indigo-500/25':
              sameSemester(semester, selected),
          }"
          @click="switchSemester(semester)"
        >
          {{ semesterLabel(semester) }}
        </button>
      </template>

      <button
        type="button"
        class="tum-live-side-navigation-group-item hover"
        @click="showAllSemesters = !showAllSemesters"
      >
        <i class="fa-solid" :class="showAllSemesters ? 'fa-chevron-up' : 'fa-chevron-down'"></i>
        <span>{{ showAllSemesters ? "Show less" : "Show all" }}</span>
      </button>
    </article>

    <article v-if="courses.pinnedCourses.length > 0" class="tum-live-side-navigation-group">
      <header>
        <i class="fa-solid fa-thumbtack"></i>
        Pinned Courses
      </header>
      <a
        v-for="course in courses.pinnedCourses"
        :key="course.id"
        class="tum-live-side-navigation-group-item hover"
        :href="courseUrl(course)"
        @click.prevent="openCourse(course)"
      >
        {{ course.name }}
      </a>
    </article>

    <article v-if="courses.userCourses.length > 0" class="tum-live-side-navigation-group">
      <header>
        <i class="fa-solid fa-graduation-cap"></i>
        My Courses
      </header>
      <a
        v-for="course in courses.userCourses.slice(0, USER_COURSES_SHOWN)"
        :key="course.id"
        class="tum-live-side-navigation-group-item hover"
        :href="courseUrl(course)"
        @click.prevent="openCourse(course)"
      >
        {{ course.name }}
      </a>
      <RouterLink
        v-if="courses.userCourses.length > USER_COURSES_SHOWN"
        :to="{ name: 'my-courses', query: $route.query }"
        class="tum-live-side-navigation-group-item hover"
        @click="sidenav.close()"
      >
        <i class="fa-solid fa-chevron-right"></i>
        Show all my courses
      </RouterLink>
    </article>

    <article class="tum-live-side-navigation-group grow">
      <header>
        <i class="fa-solid fa-chalkboard"></i>
        Public Courses
      </header>
      <a
        v-for="course in courses.publicCourses.slice(0, PUBLIC_COURSES_SHOWN)"
        :key="course.id"
        class="tum-live-side-navigation-group-item hover"
        :href="courseUrl(course)"
        @click.prevent="openCourse(course)"
      >
        {{ course.name }}
      </a>
      <RouterLink
        v-if="courses.publicCourses.length > PUBLIC_COURSES_SHOWN"
        :to="{ name: 'public-courses', query: $route.query }"
        class="tum-live-side-navigation-group-item hover"
        @click="sidenav.close()"
      >
        <i class="fa-solid fa-chevron-right"></i>
        Show all public courses
      </RouterLink>
    </article>

    <AppFooter only="mobile" />
  </section>
</template>
