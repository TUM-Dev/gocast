<script setup lang="ts">
import AuditLog from "@/components/admin/AuditLog.vue";

/**
 * What was changed about this course and its lectures, and by whom. Shown to the
 * course's administrators, who are the ones the log answers to.
 */
defineProps<{ courseId: number }>();
</script>

<template>
  <section class="rounded-lg border p-4 dark:border-gray-800" aria-labelledby="cs-activity">
    <h2 id="cs-activity" class="text-1 mb-3 text-lg font-semibold">Activity</h2>
    <AuditLog :course-id="courseId" />
  </section>
</template>
