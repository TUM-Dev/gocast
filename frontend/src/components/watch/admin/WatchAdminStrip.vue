<script setup lang="ts">
import { computed, ref } from "vue";

import type { WatchAdmin } from "@/gen/server/apiv2_pb";
import { adminLinks } from "@/lib/watch-admin";
import CameraPresets from "./CameraPresets.vue";
import EndStreamDialog from "./EndStreamDialog.vue";

/**
 * The lecturer's controls under the watch page's player: edit the lecture, its
 * statistics, the live management page, the camera presets and "End stream". Shown by
 * the page only when StreamWatch.admin is present, so the server has already decided
 * this caller administers the course; every action behind it checks again.
 *
 * The presets and the end button are the live stream's, and come empty / disallowed
 * from the server otherwise. The management link is offered only while live, since
 * that page has nothing to show for a recording.
 *
 * `ended` is for the page to fetch the stream again: the recording appears and the
 * live player goes away.
 */
const props = defineProps<{
  admin: WatchAdmin;
  courseId: number;
  streamId: number;
  /** The course's slug, for the links. */
  slug: string;
  liveNow: boolean;
}>();
const emit = defineEmits<{ ended: [discarded: boolean] }>();

const links = computed(() => adminLinks(props.courseId, props.streamId, props.slug));
const confirming = ref(false);

const linkClass = "text-3 hover:text-1 inline-flex items-center gap-1 rounded px-2 py-1 text-sm hover:bg-gray-100 dark:hover:bg-gray-800";
</script>

<template>
  <section aria-label="Lecturer controls" class="flex flex-col gap-3 rounded-lg border border-gray-200 p-3 dark:border-gray-700" data-testid="watch-admin">
    <div class="flex flex-wrap items-center gap-1">
      <RouterLink :to="links.edit" :class="linkClass"><i class="fas fa-pen" aria-hidden="true"></i> Edit lecture</RouterLink>
      <RouterLink :to="links.stats" :class="linkClass"><i class="fas fa-chart-bar" aria-hidden="true"></i> Statistics</RouterLink>
      <RouterLink v-if="liveNow" :to="links.live" :class="linkClass"><i class="fas fa-broadcast-tower" aria-hidden="true"></i> Live management</RouterLink>
      <button
        v-if="admin.canEndStream"
        type="button"
        class="ml-auto inline-flex items-center gap-1 rounded bg-red-600 px-3 py-1 text-sm font-bold text-white hover:bg-red-500"
        title="Already finished? End the stream early."
        @click="confirming = true"
      >
        <i class="fas fa-power-off" aria-hidden="true"></i> End stream
      </button>
    </div>

    <CameraPresets :course-id="courseId" :stream-id="streamId" :presets="admin.cameraPresets" />

    <EndStreamDialog
      v-if="confirming"
      :course-id="courseId"
      :stream-id="streamId"
      @close="confirming = false"
      @ended="(discarded) => emit('ended', discarded)"
    />
  </section>
</template>
