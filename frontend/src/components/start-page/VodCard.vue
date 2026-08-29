<script setup lang="ts">
import { ref } from "vue";

import { useClickOutside } from "@/lib/useClickOutside";
import { watchUrl, type Stream, type StreamProgress } from "@/lib/streams";

import StreamThumbnail from "./StreamThumbnail.vue";

/**
 * One recorded lecture in a course's listing. Ported from the VOD markup in
 * web/template/home.gohtml, menu and all.
 */
const props = defineProps<{
  lecture: Stream;
  slug: string;
  progress?: StreamProgress;
  /** Whether the caller may record watch state — false when signed out. */
  signedIn: boolean;
  downloadsEnabled: boolean;
  /** The compact row layout, which drops the thumbnail. */
  listView: boolean;
}>();

const emit = defineEmits<{ "toggle-watched": [Stream] }>();

const menu = ref<HTMLElement | null>(null);
const menuOpen = ref(false);
const downloadsOpen = ref(false);
const copied = ref(false);

useClickOutside(menu, () => (menuOpen.value = false));

/** Matches Stream.FriendlyDateStart in web/ts/api/courses.ts. */
function friendlyDate(date: Date): string {
  return date.toLocaleString("default", {
    weekday: "long",
    year: "numeric",
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

async function copyHls(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.lecture.hlsUrl);
    copied.value = true;
    setTimeout(() => (copied.value = false), 2000);
  } finally {
    menuOpen.value = false;
  }
}

function toggleWatched(): void {
  emit("toggle-watched", props.lecture);
  menuOpen.value = false;
}
</script>

<template>
  <article class="tum-live-stream group col-span-full sm:col-span-1">
    <!-- Full navigation: the watch page is still server-rendered. -->
    <a :href="watchUrl(slug, lecture.id)" class="mb-2 block">
      <StreamThumbnail v-if="!listView" :stream="lecture" :slug="slug" :progress="progress" />
    </a>

    <div class="relative flex min-h-[32px] justify-between px-1">
      <div class="w-full">
        <div v-if="lecture.name" class="flex">
          <i
            v-if="!lecture.isPubliclyVisible"
            title="This lecture is hidden"
            class="fas fa-eye-slash px-1"
            style="line-height: revert"
          ></i>
          <a class="title overflow-hidden" :href="watchUrl(slug, lecture.id)" :title="lecture.name">
            {{ lecture.name }}
          </a>
        </div>
        <span v-if="lecture.name" class="date text-sm">{{ friendlyDate(lecture.start) }}</span>

        <div v-else>
          <i
            v-if="!lecture.isPubliclyVisible"
            title="This lecture is hidden"
            class="fas fa-eye-slash px-1"
            style="line-height: revert"
          ></i>
          <a :class="listView ? 'title overflow-hidden' : ''" :href="watchUrl(slug, lecture.id)">
            <span class="date" :class="listView ? '' : 'text-sm'">
              {{ friendlyDate(lecture.start) }}
            </span>
          </a>
        </div>
      </div>

      <div ref="menu">
        <button
          type="button"
          class="px-2 md:opacity-0 md:group-hover:opacity-100"
          title="Lecture options"
          @click="menuOpen = !menuOpen"
        >
          <i class="fa-solid fa-ellipsis-vertical"></i>
        </button>

        <div
          v-if="menuOpen"
          class="tum-live-menu absolute bottom-full right-0 z-50 h-fit w-56 overflow-hidden py-px"
        >
          <button
            v-if="signedIn"
            type="button"
            class="tum-live-menu-item"
            @click="toggleWatched"
          >
            <i class="fa-solid mr-4" :class="progress?.watched ? 'fa-eye-slash' : 'fa-eye'"></i>
            <span class="block">
              {{ progress?.watched ? "Unmark as watched" : "Mark as watched" }}
            </span>
          </button>

          <button type="button" class="tum-live-menu-item" @click="copyHls">
            <i class="fa-solid fa-link mr-4"></i>
            <span class="block">Copy HLS URL</span>
          </button>

          <div v-if="signedIn && downloadsEnabled && lecture.downloads.length > 0">
            <button
              type="button"
              class="tum-live-menu-item"
              @click="downloadsOpen = !downloadsOpen"
            >
              <i class="fa-solid fa-cloud-arrow-down mr-4"></i>
              <span class="block">Download</span>
              <i
                class="fa-solid ml-auto"
                :class="downloadsOpen ? 'fa-chevron-up' : 'fa-chevron-down'"
              ></i>
            </button>
            <div v-if="downloadsOpen" class="grid gap-1">
              <a
                v-for="file in lecture.downloads"
                :key="file.downloadUrl"
                :href="file.downloadUrl"
                download
                class="block px-4 py-2 text-left hover:bg-gray-100 dark:hover:bg-gray-800"
              >
                {{ file.friendlyName }}
              </a>
            </div>
          </div>
        </div>
      </div>
    </div>

    <span v-if="copied" class="text-5 px-1 text-xs">Copied.</span>
  </article>
</template>
