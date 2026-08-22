<script setup lang="ts">
import CourseListView from "@/components/CourseListView.vue";
import StartPageLayout from "@/components/StartPageLayout.vue";
import { useCourseStore } from "@/stores/courses";

const courses = useCourseStore();
</script>

<template>
  <StartPageLayout>
    <CourseListView
      id="user-courses"
      title="My Courses"
      icon="fa-solid fa-graduation-cap"
      :courses="courses.userCourses"
    />
  </StartPageLayout>
</template>
