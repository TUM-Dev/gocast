<script setup lang="ts">
import { courseUrl, isHidden, type LiveStream } from "@/lib/courses";
import { watchUrl } from "@/lib/streams";

import StreamThumbnail from "./StreamThumbnail.vue";

/**
 * A lecture that is live right now. Ported from the `#livestreams` markup in
 * web/template/home.gohtml.
 *
 * The viewer count the old card carried is deliberately absent: the number lives in
 * the v1 chat session map and GetLiveCourses has no way to reach it yet.
 */
defineProps<{ livestream: LiveStream }>();

/** Matches Stream.UntilString in web/ts/api/courses.ts. */
function untilString(end: Date): string {
  const minutes = end.getMinutes();
  return `Until ${end.getHours()}:${minutes < 10 ? `${minutes}0` : minutes}`;
}
</script>

<template>
  <article :id="`livestream-${livestream.stream.id}`" class="tum-live-stream col-span-full p-3 lg:col-span-1">
    <div class="relative mb-2 aspect-video">
      <div
        class="absolute right-2 top-2 z-40 flex items-center space-x-2 text-xs font-semibold text-white"
      >
        <span v-if="isHidden(livestream.course)" class="tum-live-badge bg-neutral-700">Hidden</span>
        <a
          v-if="livestream.lectureHall"
          class="tum-live-badge bg-black"
          target="_blank"
          rel="noopener"
          :href="livestream.lectureHall.externalUrl"
        >
          <i class="fas fa-location-pin"></i>
          <span>{{ livestream.lectureHall.name }}</span>
        </a>
      </div>

      <!-- Full navigation: the watch page is still server-rendered. -->
      <a :href="watchUrl(livestream.course.slug, livestream.stream.id)">
        <StreamThumbnail
          class="h-full"
          bare
          :stream="livestream.stream"
          :slug="livestream.course.slug"
        />
      </a>
    </div>

    <div class="px-2">
      <RouterLink class="course text-sm" :to="courseUrl(livestream.course)">
        {{ livestream.course.name }}
      </RouterLink>
      <a
        v-if="livestream.stream.name"
        class="title"
        :href="watchUrl(livestream.course.slug, livestream.stream.id)"
      >
        {{ livestream.stream.name }}
      </a>
      <span class="date text-sm">{{ untilString(livestream.stream.end) }}</span>
    </div>
  </article>
</template>
