<script setup lang="ts">
import AttachmentsEditor from "@/components/admin/lectures/content/AttachmentsEditor.vue";
import RecordingPanel from "@/components/admin/lectures/content/RecordingPanel.vue";
import SectionsEditor from "@/components/admin/lectures/content/SectionsEditor.vue";
import ThumbnailEditor from "@/components/admin/lectures/content/ThumbnailEditor.vue";
import type { CourseLecture } from "@/lib/course-lectures";

/**
 * A lecture's content, inside its editor: the recording, its chapters, the files
 * that go with it and its thumbnail. Each part saves on its own and asks the list to
 * reload, which hands the changed lecture back down.
 */
defineProps<{
  courseId: number;
  lecture: CourseLecture;
}>();

const emit = defineEmits<{ changed: [] }>();
</script>

<template>
  <RecordingPanel :course-id="courseId" :lecture="lecture" @changed="emit('changed')" />
  <SectionsEditor
    :course-id="courseId"
    :lecture-id="lecture.id"
    :sections="lecture.videoSections"
    @changed="emit('changed')"
  />
  <AttachmentsEditor
    :course-id="courseId"
    :lecture-id="lecture.id"
    :files="lecture.files"
    @changed="emit('changed')"
  />
  <ThumbnailEditor :course-id="courseId" :lecture-id="lecture.id" :files="lecture.files" @changed="emit('changed')" />
</template>
