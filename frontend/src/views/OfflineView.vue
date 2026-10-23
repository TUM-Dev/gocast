<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";

import VideoPlayer from "@/components/player/VideoPlayer.vue";
import { clipOf, expiryText, formatSize } from "@/lib/offline/library";
import { playlistUrl } from "@/lib/offline/media";
import { online } from "@/lib/pwa";
import { singleQueryParam } from "@/lib/route-query";
import { useOfflineStore } from "@/stores/offline";

/**
 * The lectures saved on this device, and a player for them. Nothing here asks the
 * server: it has to work on a train. Which lecture plays is `?play=<stream id>`, so the
 * back button closes the player.
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

function play(streamId: number) {
  router.push({ query: { play: String(streamId) } });
}

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

function lectureDate(start: number): string {
  return start ? new Date(start).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" }) : "";
}

onMounted(async () => {
  await offline.load();
  measure();
  // Online, an expired lecture may only need renewing.
  if (online.value) await offline.sync().catch(() => {});
  now.value = Date.now();
});
</script>

<template>
  <section class="text-3 mx-auto flex w-full max-w-4xl flex-col gap-4 p-4">
    <header class="flex flex-col gap-1">
      <h1 class="text-1 text-2xl font-bold">Offline lectures</h1>
      <p class="text-5 text-sm">
        Lectures saved on this device play without a connection. Each one has to be renewed online once a week.
        <span v-if="usage"> {{ formatSize(usage.usage) }} of {{ formatSize(usage.quota) }} used.</span>
      </p>
    </header>

    <!-- The saved copy is decrypted while it plays; there is nothing to save from here. -->
    <div v-if="playing" class="flex flex-col gap-2" data-testid="offline-player" @contextmenu.prevent>
      <VideoPlayer
        :key="playing.streamId"
        :src="playlistUrl(playing.streamId)"
        :clip="clipOf(playing)"
        persist
        hotkeys="page"
      />
      <div class="flex items-center justify-between gap-2">
        <div>
          <h2 class="text-1 font-semibold">{{ playing.title }}</h2>
          <p class="text-5 text-sm">{{ playing.courseName }} · {{ lectureDate(playing.start) }}</p>
        </div>
        <button type="button" class="tum-live-button tum-live-button-secondary" @click="router.push({ query: {} })">Close</button>
      </div>
    </div>

    <p v-if="offline.lectures.length === 0" class="text-5" data-testid="offline-empty">
      No lectures are saved on this device. Open a lecture and choose "Save offline" from its downloads.
    </p>

    <ul v-if="saved.length > 0" class="flex flex-col divide-y divide-gray-200 dark:divide-gray-700" data-testid="offline-list">
      <li v-for="l in saved" :key="l.streamId" class="flex items-center gap-3 py-3">
        <button
          type="button"
          class="flex grow flex-col items-start text-left disabled:opacity-60"
          :disabled="l.expiresAt <= now"
          @click="play(l.streamId)"
        >
          <span class="text-1 font-semibold">{{ l.title }}</span>
          <span class="text-5 text-sm">{{ l.courseName }} · {{ lectureDate(l.start) }} · {{ formatSize(l.sizeBytes) }}</span>
          <span class="text-5 text-xs" :class="l.expiresAt <= now && 'text-red-600 dark:text-red-400'">
            {{ expiryText(l.expiresAt, now) }}
          </span>
        </button>
        <button type="button" class="text-4 hover:text-1 p-2" :aria-label="`Remove ${l.title}`" @click="remove(l.streamId)">
          <i class="fa-solid fa-trash"></i>
        </button>
      </li>
    </ul>

    <section v-if="unfinished.length > 0" class="flex flex-col gap-2">
      <h2 class="text-1 font-semibold">Not finished</h2>
      <ul class="flex flex-col divide-y divide-gray-200 dark:divide-gray-700">
        <li v-for="l in unfinished" :key="l.streamId" class="flex items-center gap-3 py-3">
          <span class="flex grow flex-col">
            <span class="text-1">{{ l.title }}</span>
            <span class="text-5 text-sm">
              {{ offline.progress[l.streamId] !== undefined ? `${Math.floor(offline.progress[l.streamId] * 100)} %` : l.courseName }}
            </span>
            <span v-if="offline.errors[l.streamId]" class="text-xs text-red-600 dark:text-red-400" role="alert">{{ offline.errors[l.streamId] }}</span>
          </span>
          <button
            type="button"
            class="tum-live-button tum-live-button-secondary"
            :disabled="!online || offline.saving[l.streamId]"
            @click="offline.save(l.streamId)"
          >
            Resume
          </button>
          <button type="button" class="text-4 hover:text-1 p-2" :aria-label="`Remove ${l.title}`" @click="remove(l.streamId)">
            <i class="fa-solid fa-trash"></i>
          </button>
        </li>
      </ul>
    </section>
  </section>
</template>
