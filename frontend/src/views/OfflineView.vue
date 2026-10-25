<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";

import OfflineThumbnail from "@/components/offline/OfflineThumbnail.vue";
import VideoPlayer from "@/components/player/VideoPlayer.vue";
import WatchLayout from "@/components/watch/WatchLayout.vue";
import type { OfflineLectureRecord } from "@/lib/offline/db";
import { fetchThumbnail } from "@/lib/offline/download";
import { clipOf, downloadFraction, expiryText, formatSize, groupByCourse, playableSeconds } from "@/lib/offline/library";
import { getThumbnail, playlistUrl } from "@/lib/offline/media";
import { online } from "@/lib/pwa";
import { singleQueryParam } from "@/lib/route-query";
import { formatStreamTime } from "@/lib/watch";
import { useOfflineStore } from "@/stores/offline";

/**
 * The lectures saved on this device, and a player for them. Nothing here asks the
 * server: it has to work on a train. Which lecture plays is `?play=<stream id>`, so the
 * back button closes the player.
 *
 * The list looks like a course's lectures (VodCard), and the player like the watch page
 * (WatchLayout), with the other saved lectures where the watch page has its panels.
 */
const offline = useOfflineStore();
const route = useRoute();
const router = useRouter();

const now = ref(Date.now());
const usage = ref<{ usage: number; quota: number } | null>(null);

const saved = computed(() => offline.lectures.filter((l) => l.state === "saved"));
const unfinished = computed(() => offline.lectures.filter((l) => l.state !== "saved"));
const playing = computed(() => {
  const id = Number(singleQueryParam(route.query, "play"));
  return saved.value.find((l) => l.streamId === id);
});
const others = computed(() => saved.value.filter((l) => l.streamId !== playing.value?.streamId));

const courses = computed(() => groupByCourse(saved.value));
// Beside the player, the playing lecture's course comes first: what is watched next is
// most likely the next lecture of the same course.
const otherCourses = computed(() => {
  const groups = groupByCourse(others.value);
  const current = playing.value && (playing.value.courseSlug || playing.value.courseName);
  const i = groups.findIndex((g) => g.key === current);
  return i > 0 ? [groups[i]!, ...groups.slice(0, i), ...groups.slice(i + 1)] : groups;
});

const playLink = (streamId: number) => ({ query: { play: String(streamId) } });
const expired = (l: OfflineLectureRecord) => l.expiresAt <= now.value;

// A lecture picked from the side list starts at the top, where its player is.
watch(
  () => playing.value?.streamId,
  (id) => id !== undefined && window.scrollTo({ top: 0 }),
);

// ------------------------------------------------------------ thumbnails

/** Object URLs of the saved thumbnails, by package; revoked when the page goes. */
const thumbnails = ref<Record<number, string>>({});
/** Looked up by package and state: an unfinished lecture gets its thumbnail once saved. */
const looked = new Set<string>();

// Lectures saved before thumbnails were kept get theirs on the next visit online.
async function loadThumbnails(lectures: readonly OfflineLectureRecord[]): Promise<void> {
  for (const l of lectures) {
    const key = `${l.packageId}:${l.state}`;
    if (looked.has(key) || thumbnails.value[l.packageId]) continue;
    looked.add(key);
    const blob =
      (await getThumbnail(l.packageId).catch(() => undefined)) ??
      (online.value && l.state === "saved" ? await fetchThumbnail(l) : undefined);
    if (blob) thumbnails.value = { ...thumbnails.value, [l.packageId]: URL.createObjectURL(blob) };
  }
}
watch(() => offline.lectures, (lectures) => void loadThumbnails(lectures));
onBeforeUnmount(() => Object.values(thumbnails.value).forEach((url) => URL.revokeObjectURL(url)));

// ------------------------------------------------------------ actions

async function remove(streamId: number) {
  if (!window.confirm("Remove this lecture from the device?")) return;
  if (playing.value?.streamId === streamId) await router.replace({ query: {} });
  await offline.remove(streamId);
  await measure();
}

async function measure() {
  const estimate = await navigator.storage?.estimate?.().catch(() => undefined);
  usage.value = estimate?.usage !== undefined && estimate.quota !== undefined ? { usage: estimate.usage, quota: estimate.quota } : null;
}

const usedPercent = computed(() => (usage.value ? Math.min(100, (usage.value.usage / usage.value.quota) * 100) : 0));

function lectureDate(start: number): string {
  return start ? formatStreamTime(new Date(start)) : "";
}

onMounted(async () => {
  await offline.load();
  void loadThumbnails(offline.lectures);
  measure();
  // Online, an expired lecture may only need renewing.
  if (online.value) await offline.sync().catch(() => {});
  now.value = Date.now();
});
</script>

<template>
  <!-- The player, laid out like the watch page. -->
  <WatchLayout v-if="playing" :sidebar-open="others.length > 0">
    <template #player>
      <div
        v-if="expired(playing)"
        class="flex aspect-video w-full flex-col items-center justify-center gap-2 bg-gray-900 p-6 text-center text-white"
        data-testid="offline-expired"
      >
        <i class="fa-solid fa-lock text-2xl" aria-hidden="true"></i>
        <p class="text-xl font-semibold">This lecture has to be renewed.</p>
        <p class="text-sm text-gray-300">Go online once, and it plays offline for another week.</p>
      </div>
      <!-- The saved copy is decrypted while it plays; there is nothing to save from here. -->
      <div v-else data-testid="offline-player" @contextmenu.prevent>
        <VideoPlayer
          :key="playing.streamId"
          :src="playlistUrl(playing.streamId)"
          :clip="clipOf(playing)"
          persist
          hotkeys="page"
        />
      </div>
    </template>

    <template #info>
      <div class="flex flex-col gap-1">
        <h1 class="text-1 line-clamp-2 text-lg font-bold lg:text-2xl" data-testid="offline-title">{{ playing.title }}</h1>
        <p v-if="playing.courseName" class="text-5 text-sm lg:text-base">{{ playing.courseName }}</p>
        <p class="text-5 flex flex-wrap items-center gap-x-2 text-xs lg:text-sm">
          <span class="flex items-center gap-1 rounded bg-green-600/10 px-2 py-0.5 text-xs font-semibold text-green-700 dark:text-green-400">
            <i class="fa-solid fa-circle-check" aria-hidden="true"></i>Saved on this device
          </span>
          <span v-if="playing.start">{{ lectureDate(playing.start) }}</span>
          <span v-if="playing.start" aria-hidden="true">&middot;</span>
          <span :class="expired(playing) && 'text-red-600 dark:text-red-400'">{{ expiryText(playing.expiresAt, now) }}</span>
        </p>
      </div>
    </template>

    <template #action-bar>
      <div class="flex flex-wrap items-center gap-2">
        <RouterLink :to="{ query: {} }" class="tum-live-button tum-live-button-tertiary text-xs" data-testid="offline-back">
          <i class="fa-solid fa-arrow-left" aria-hidden="true"></i>All offline lectures
        </RouterLink>
        <button type="button" class="tum-live-button tum-live-button-tertiary text-xs" @click="remove(playing.streamId)">
          <i class="fa-solid fa-trash" aria-hidden="true"></i>Remove from device
        </button>
      </div>
    </template>

    <template #sidebar>
      <div class="flex h-full min-h-0 flex-col">
        <h2 class="text-1 border-b px-4 py-3 text-sm font-semibold dark:border-gray-800">More on this device</h2>
        <div class="min-h-0 grow overflow-y-auto p-2" data-testid="offline-up-next">
          <section v-for="course in otherCourses" :key="course.key" class="pb-2">
            <h3 class="text-5 truncate px-2 pt-2 pb-1 text-xs font-semibold tracking-wide uppercase">{{ course.name }}</h3>
            <ul>
              <li v-for="l in course.lectures" :key="l.streamId">
                <RouterLink
                  :to="playLink(l.streamId)"
                  class="group flex gap-3 rounded-lg p-2 transition-colors hover:bg-gray-100 focus-visible:bg-gray-100 dark:hover:bg-gray-800 dark:focus-visible:bg-gray-800"
                >
                  <div class="w-36 shrink-0">
                    <OfflineThumbnail :src="thumbnails[l.packageId]" :expired="expired(l)" compact />
                  </div>
                  <div class="flex min-w-0 flex-col">
                    <span class="text-1 line-clamp-2 text-sm font-medium group-hover:underline">{{ l.title }}</span>
                    <span v-if="l.start" class="text-5 text-xs">{{ lectureDate(l.start) }}</span>
                    <span class="text-5 text-xs" :class="expired(l) && 'text-red-600 dark:text-red-400'">
                      {{ expiryText(l.expiresAt, now) }}
                    </span>
                  </div>
                </RouterLink>
              </li>
            </ul>
          </section>
        </div>
      </div>
    </template>
  </WatchLayout>

  <!-- The library. -->
  <section v-else class="text-3 mx-auto flex w-full max-w-6xl flex-col gap-6 p-4">
    <header class="flex flex-col gap-2">
      <h1 class="text-1 text-2xl font-bold">Offline lectures</h1>
      <p class="text-5 text-sm">
        Lectures saved on this device play without a connection. Each one has to be renewed online once a week.
      </p>
      <div v-if="usage" class="flex max-w-xs flex-col gap-1">
        <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
          <div class="tum-live-bg-primary h-full rounded-full" :style="{ width: `${usedPercent}%` }"></div>
        </div>
        <span class="text-5 text-xs">{{ formatSize(usage.usage) }} of {{ formatSize(usage.quota) }} used</span>
      </div>
    </header>

    <div
      v-if="offline.lectures.length === 0"
      class="flex flex-col items-center gap-3 rounded-lg border border-dashed px-6 py-12 text-center dark:border-gray-700"
      data-testid="offline-empty"
    >
      <i class="fa-solid fa-cloud-arrow-down text-4 text-3xl" aria-hidden="true"></i>
      <p class="text-1 font-semibold">No lectures are saved on this device.</p>
      <p class="text-5 max-w-md text-sm">Open a lecture and choose "Save offline" from its downloads.</p>
    </div>

    <div v-if="saved.length > 0" class="flex flex-col gap-8" data-testid="offline-list">
      <section v-for="course in courses" :key="course.key" class="flex flex-col gap-3" data-testid="offline-course">
        <h2 class="text-1 flex items-baseline gap-2 text-lg font-semibold">
          <span class="truncate">{{ course.name }}</span>
          <span class="text-5 shrink-0 text-sm font-normal">
            {{ course.lectures.length }} lecture{{ course.lectures.length === 1 ? "" : "s" }}
          </span>
        </h2>
        <ul class="grid grid-cols-1 gap-x-4 gap-y-6 sm:grid-cols-2 lg:grid-cols-3">
          <li v-for="l in course.lectures" :key="l.streamId" class="group/card flex flex-col gap-2">
            <RouterLink :to="playLink(l.streamId)" class="group block rounded-lg" :aria-label="`Play ${l.title}`">
              <OfflineThumbnail :src="thumbnails[l.packageId]" :seconds="playableSeconds(l)" :expired="expired(l)" />
            </RouterLink>
            <div class="flex items-start justify-between gap-2 px-1">
              <div class="flex min-w-0 flex-col">
                <RouterLink :to="playLink(l.streamId)" class="text-1 line-clamp-2 font-medium hover:underline" :title="l.title">
                  {{ l.title }}
                </RouterLink>
                <span class="text-5 text-xs font-light">
                  {{ lectureDate(l.start) }}<template v-if="l.start"> &middot; </template>{{ formatSize(l.sizeBytes) }}
                </span>
                <span class="text-5 text-xs" :class="expired(l) && 'text-red-600 dark:text-red-400'">
                  {{ expiryText(l.expiresAt, now) }}
                </span>
              </div>
              <!-- Revealed on hover like the lecture cards' menu; always there on a phone. -->
              <button
                type="button"
                class="text-4 hover:text-1 shrink-0 rounded p-2 transition-opacity focus-visible:opacity-100 md:opacity-0 md:group-hover/card:opacity-100"
                :aria-label="`Remove ${l.title}`"
                title="Remove from device"
                @click="remove(l.streamId)"
              >
                <i class="fa-solid fa-trash"></i>
              </button>
            </div>
          </li>
        </ul>
      </section>
    </div>

    <section v-if="unfinished.length > 0" class="flex flex-col gap-2">
      <h2 class="text-1 font-semibold">Not finished</h2>
      <ul class="flex flex-col divide-y divide-gray-200 dark:divide-gray-700">
        <li v-for="l in unfinished" :key="l.streamId" class="flex items-center gap-3 py-3">
          <div class="w-28 shrink-0 opacity-70">
            <div class="tum-live-thumbnail mb-0 aspect-video overflow-hidden">
              <img v-if="thumbnails[l.packageId]" :src="thumbnails[l.packageId]" alt="" class="h-full w-full object-cover" />
            </div>
          </div>
          <div class="flex min-w-0 grow flex-col gap-1">
            <span class="text-1 truncate">{{ l.title }}</span>
            <span class="text-5 truncate text-xs">{{ l.courseName }}</span>
            <div class="flex items-center gap-2">
              <div class="h-1.5 w-full max-w-48 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
                <div
                  class="tum-live-bg-primary h-full rounded-full transition-[width] duration-300"
                  :style="{ width: `${downloadFraction(l, offline.progress[l.streamId]) * 100}%` }"
                ></div>
              </div>
              <span class="text-5 text-xs">{{ Math.floor(downloadFraction(l, offline.progress[l.streamId]) * 100) }} %</span>
            </div>
            <span v-if="offline.errors[l.streamId]" class="text-xs text-red-600 dark:text-red-400" role="alert">{{ offline.errors[l.streamId] }}</span>
          </div>
          <button
            type="button"
            class="tum-live-button tum-live-button-secondary shrink-0"
            :disabled="!online || offline.saving[l.streamId]"
            @click="offline.save(l.streamId)"
          >
            {{ offline.saving[l.streamId] ? "Saving…" : "Resume" }}
          </button>
          <button
            type="button"
            class="text-4 hover:text-1 shrink-0 p-2"
            :aria-label="`Remove ${l.title}`"
            title="Remove from device"
            @click="remove(l.streamId)"
          >
            <i class="fa-solid fa-trash"></i>
          </button>
        </li>
      </ul>
    </section>
  </section>
</template>
