<script setup lang="ts">
import CourseListView from "@/components/CourseListView.vue";
import StartPageLayout from "@/components/StartPageLayout.vue";
import { useCourseStore } from "@/stores/courses";

const courses = useCourseStore();
</script>

<template>
  <StartPageLayout>
    <CourseListView
      id="public-courses"
      title="Public Courses"
      icon="fa-solid fa-chalkboard"
      :courses="courses.publicCourses"
    />
  </StartPageLayout>
</template>
