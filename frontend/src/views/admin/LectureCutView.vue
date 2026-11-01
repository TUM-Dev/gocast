<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef, watch } from "vue";
import { useRoute } from "vue-router";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import CutTimeline from "@/components/admin/lectures/cut/CutTimeline.vue";
import TranscodingProgress from "@/components/admin/lectures/content/TranscodingProgress.vue";
import VideoPlayer from "@/components/player/VideoPlayer.vue";
import { ApiError } from "@/lib/api";
import {
  acceptAll,
  acceptCut,
  addCut,
  appliedCuts,
  cutAt,
  formatTime,
  keptSeconds,
  leavesEnough,
  removeCut,
  removedSeconds,
  resizeCut,
  suggestCuts,
  toRanges,
  type Cut,
} from "@/lib/cuts";
import { fetchTranscodingProgress, type TranscodingProgress as Progress } from "@/lib/lecture-content";
import {
  LectureCutMode,
  abandonCut,
  cutErrorMessage,
  cutLecture,
  fetchOpenCut,
  fetchWaveform,
  pollWaveform,
  requestWaveform,
  retryCut,
  waveformDuration,
  type OpenCut,
  type Waveform,
} from "@/lib/lecture-cut";
import { fetchStreamWatch, viewOfKind, WatchViewKind } from "@/lib/watch";
import { redirectToLogin, useAuthStore } from "@/stores/auth";
import { useCourseAdminStore } from "@/stores/course-admin";

/**
 * The cut editor: remove stretches from a lecture's recording. The player previews the
 * recording, the timeline below it shows the audio levels and takes the cuts, and the
 * silences a runner found are proposed as cuts the person may accept.
 *
 * Nothing is cut here. Submitting hands the ranges to the server, which has runners
 * re-encode every version of the recording and either replace it or make a copy of the
 * lecture. As with every admin page the server decides who may do any of this.
 *
 * A cut stays open on the server until every version is cut. While one is, the page shows
 * how far it is instead of the editor, and for a version that failed offers to cut it again
 * with the same ranges or to give the cut up.
 */
const auth = useAuthStore();
const courses = useCourseAdminStore();
const route = useRoute();

const courseId = computed(() => Number(route.params.courseID));
const streamId = computed(() => Number(route.params.streamID));

type Phase = "loading" | "building" | "ready" | "converting" | "cutting" | "failed";
const phase = ref<Phase>("loading");
const error = ref("");
const lectureName = ref("");
const slug = ref("");
const src = ref("");
const waveform = shallowRef<Waveform | null>(null);
const duration = ref(0);
/** The versions of the lecture that are being converted, by an upload say. */
const progresses = ref<Progress[]>([]);

const cuts = ref<Cut[]>([]);
const selectedId = ref<number | null>(null);
const pendingStart = ref<number | null>(null);
const currentTime = ref(0);
const zoom = ref(1);
const skipCuts = ref(true);

const player = ref<InstanceType<typeof VideoPlayer> | null>(null);

/* Loading. */

let stopPolling: (() => void) | null = null;
onBeforeUnmount(() => stopPolling?.());

function fail(err: unknown): void {
  error.value = cutErrorMessage(err);
  phase.value = "failed";
}

/* A cut that is not finished. */

const openCut = ref<OpenCut | null>(null);
const outcome = ref<"" | "done">("");
const notice = ref("");
const working = ref(false);
const confirmingAbandon = ref(false);
let leaving = false;
onBeforeUnmount(() => (leaving = true));

const cutRunning = computed(() => openCut.value?.progresses.some((p) => !p.failed) ?? false);
const cutFailed = computed(() => openCut.value?.progresses.some((p) => p.failed) ?? false);

function showOpenCut(open: OpenCut): void {
  openCut.value = open;
  outcome.value = "";
  phase.value = "cutting";
  // No version is left at all: the server is finishing the cut. Versions being cut report to
  // the progress below, which says when they are done, and failed ones are for the person.
  if (open.progresses.length === 0) void settle();
}

/**
 * After the versions are done the server finishes the cut, which closes it. Asks until it
 * has, unless a version failed, which is for the person to decide.
 */
async function settle(): Promise<void> {
  for (let attempt = 0; attempt < SETTLE_TRIES && !leaving; attempt++) {
    let open: OpenCut | null | undefined;
    try {
      open = await fetchOpenCut(courseId.value, streamId.value);
    } catch {
      open = undefined; // a dropped request is asked again
    }
    if (open === null) {
      outcome.value = "done";
      return;
    }
    if (open) {
      openCut.value = open;
      if (open.progresses.length > 0) return; // failed, or being cut again
    }
    await new Promise((resolve) => setTimeout(resolve, SETTLE_MS));
  }
}

const SETTLE_TRIES = 15;
const SETTLE_MS = 2000;

async function retry(): Promise<void> {
  working.value = true;
  error.value = "";
  try {
    await retryCut(courseId.value, streamId.value);
    await load(courseId.value, streamId.value);
  } catch (err) {
    error.value = cutErrorMessage(err);
  } finally {
    working.value = false;
  }
}

async function abandon(): Promise<void> {
  confirmingAbandon.value = false;
  working.value = true;
  error.value = "";
  try {
    const left = await abandonCut(courseId.value, streamId.value);
    notice.value = left.length
      ? `The cut was given up. The ${left.join(" and ")} ${left.length === 1 ? "version was" : "versions were"} not cut and still ${left.length === 1 ? "holds" : "hold"} what you cut out of the others.`
      : "The cut was given up.";
    await load(courseId.value, streamId.value);
  } catch (err) {
    error.value = cutErrorMessage(err);
  } finally {
    working.value = false;
  }
}

function useWaveform(w: Waveform): void {
  waveform.value = w;
  duration.value = waveformDuration(w);
  cuts.value = suggestCuts(w.silences, duration.value);
  phase.value = "ready";
}

async function load(course: number, stream: number): Promise<void> {
  stopPolling?.();
  phase.value = "loading";
  error.value = "";
  waveform.value = null;
  cuts.value = [];
  try {
    await courses.load(course);
    const loaded = courses.course;
    if (!loaded) {
      error.value = courses.error || "This course does not exist or you do not administer it.";
      phase.value = "failed";
      return;
    }
    slug.value = loaded.slug;

    const watchData = await fetchStreamWatch(loaded.slug, stream);
    const view = viewOfKind(watchData, WatchViewKind.COMBINED) ?? watchData.views[0];
    lectureName.value = watchData.stream?.displayName ?? "";
    if (!view || watchData.stream?.liveNow) {
      error.value = "This lecture has no finished recording to cut.";
      phase.value = "failed";
      return;
    }
    src.value = view.playlistUrl;

    const open = await fetchOpenCut(course, stream);
    if (open) {
      showOpenCut(open);
      return;
    }

    // Cutting while a conversion runs would race it for the same files.
    progresses.value = await fetchTranscodingProgress(course, stream);
    if (progresses.value.some((p) => !p.failed)) {
      phase.value = "converting";
      return;
    }

    const ready = await fetchWaveform(course, stream);
    if (ready) {
      useWaveform(ready);
      return;
    }
    phase.value = "building";
    await requestWaveform(course, stream);
    stopPolling = pollWaveform({
      fetch: () => fetchWaveform(course, stream),
      onReady: useWaveform,
      onError: fail,
    });
  } catch (err) {
    if (err instanceof ApiError && err.isUnauthenticated) {
      redirectToLogin();
      return;
    }
    fail(err);
  }
}

watch(
  [courseId, streamId],
  async ([course, stream]) => {
    const user = await auth.load().catch(() => null);
    if (!user) {
      redirectToLogin();
      return;
    }
    await load(course, stream);
  },
  { immediate: true },
);

/* The player. */

function onTime(t: number): void {
  currentTime.value = t;
  // Only while playing: a click into a cut to look at it must not bounce out again.
  if (!skipCuts.value || player.value?.isPaused()) return;
  const inside = cutAt(cuts.value, t);
  if (inside) player.value?.seek(inside.end);
}

function seek(t: number): void {
  player.value?.seek(t);
  currentTime.value = t;
}

/* The cuts. */

const applied = computed(() => appliedCuts(cuts.value));
const suggestions = computed(() => cuts.value.filter((c) => c.suggested));
const removed = computed(() => removedSeconds(cuts.value));
const kept = computed(() => keptSeconds(cuts.value, duration.value));
const versionsDiffer = computed(() => waveform.value?.versionsDiffer ?? false);

function markStart(): void {
  pendingStart.value = currentTime.value;
}

function markEnd(): void {
  if (pendingStart.value === null) return;
  cuts.value = addCut(cuts.value, pendingStart.value, currentTime.value, duration.value);
  pendingStart.value = null;
}

function create(start: number, end: number): void {
  cuts.value = addCut(cuts.value, start, end, duration.value);
}

function resize(id: number, edge: "start" | "end", to: number): void {
  cuts.value = resizeCut(cuts.value, id, edge, to, duration.value);
}

function remove(id: number): void {
  cuts.value = removeCut(cuts.value, id);
  if (selectedId.value === id) selectedId.value = null;
}

function dismissSuggestions(): void {
  cuts.value = appliedCuts(cuts.value);
}

const zoomIn = () => (zoom.value = Math.min(16, zoom.value * 2));
const zoomOut = () => (zoom.value = Math.max(1, zoom.value / 2));

/* Submitting. */

const mode = ref<LectureCutMode>(LectureCutMode.OVERWRITE);
const confirming = ref(false);
const submitting = ref(false);

const canSubmit = computed(
  () => applied.value.length > 0 && leavesEnough(cuts.value, duration.value) && !versionsDiffer.value && !submitting.value,
);

function submitClicked(): void {
  // Replacing the recording can't be taken back, so it asks once more.
  if (mode.value === LectureCutMode.OVERWRITE) {
    confirming.value = true;
    return;
  }
  void submit();
}

async function submit(): Promise<void> {
  confirming.value = false;
  submitting.value = true;
  error.value = "";
  notice.value = "";
  try {
    await cutLecture(courseId.value, streamId.value, mode.value, toRanges(cuts.value));
  } catch (err) {
    const message = cutErrorMessage(err);
    // Some versions may have been started: what the server has is what the page shows.
    if (err instanceof ApiError && err.status === 503) await load(courseId.value, streamId.value);
    error.value = message;
    submitting.value = false;
    return;
  }
  try {
    await load(courseId.value, streamId.value);
  } finally {
    submitting.value = false;
  }
}

const watchPath = computed(() =>
  openCut.value ? `/w/${encodeURIComponent(slug.value)}/${openCut.value.targetStreamId}` : "",
);

/** Loads the lecture again, once a conversion it waited for is done. */
function reload(): void {
  void load(courseId.value, streamId.value);
}
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-4">
      <header>
        <h1 class="text-1 text-2xl font-bold">Cut video</h1>
        <p class="text-3">
          <RouterLink :to="`/admin/courses/${courseId}/lectures`" class="hover:underline">
            {{ courses.course?.name ?? "Lectures" }}
          </RouterLink>
          <template v-if="lectureName"> · {{ lectureName }}</template>
        </p>
      </header>

      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>
      <p v-if="notice" class="rounded-lg bg-info/25 px-2 py-2 text-sm" role="status" data-testid="cut-notice">{{ notice }}</p>

      <p v-if="phase === 'loading'" class="text-5 text-sm">Loading the lecture…</p>

      <div v-else-if="phase === 'converting'" class="flex flex-col gap-2">
        <p class="text-sm">The lecture is being converted. It can be cut once that is done.</p>
        <TranscodingProgress
          :course-id="courseId"
          :lecture-id="streamId"
          converting
          :progresses="progresses"
          @done="reload"
        />
      </div>

      <section v-else-if="phase === 'cutting' && openCut" class="flex flex-col gap-3" data-testid="cut-progress">
        <h2 class="text-1 text-lg font-semibold">
          {{
            outcome === "done"
              ? "The video is cut."
              : cutRunning
                ? "Cutting the video…"
                : cutFailed
                  ? "The cut did not finish."
                  : "Finishing the cut…"
          }}
        </h2>
        <p v-if="outcome !== 'done' && !cutRunning && !cutFailed" class="text-5 flex items-center gap-2 text-sm" role="status">
          <i class="fas fa-circle-notch fa-spin"></i>Moving the sections of the lecture along.
        </p>
        <TranscodingProgress
          v-if="outcome !== 'done'"
          :course-id="courseId"
          :lecture-id="openCut.targetStreamId"
          converting
          :progresses="openCut.progresses"
          retry-hint="Cut it again, or give up the cut."
          @done="settle"
        />

        <div v-if="cutFailed && !cutRunning" class="flex flex-col gap-2 text-sm" data-testid="cut-failed">
          <p class="text-2">
            <template v-if="openCut.duplicate">
              The versions that are done are kept. Cutting again runs only the ones that failed, with the same cuts.
            </template>
            <template v-else>
              The versions that are done are cut already. Cutting again runs only the ones that failed, with the same cuts,
              so nothing is cut twice.
            </template>
          </p>
          <div v-if="confirmingAbandon" class="flex flex-wrap items-center gap-2 rounded-lg bg-danger/25 px-2 py-2" role="alertdialog">
            <span v-if="openCut.duplicate">This removes the new lecture. The original is not changed.</span>
            <span v-else>
              The versions that are cut stay cut and can't be put back. The ones that failed keep what you cut out of the others.
            </span>
            <button type="button" class="tum-live-button text-white bg-red-600" data-testid="cut-abandon-confirm" @click="abandon">
              Give up the cut
            </button>
            <button type="button" class="tum-live-button tum-live-button-secondary" @click="confirmingAbandon = false">
              Keep it
            </button>
          </div>
          <div v-else class="flex flex-wrap gap-2">
            <button
              type="button"
              class="tum-live-button text-white bg-blue-500 disabled:opacity-50 dark:bg-indigo-600"
              :disabled="working"
              data-testid="cut-retry"
              @click="retry"
            >
              <i class="fas fa-arrows-rotate mr-1"></i>Cut the failed versions again
            </button>
            <button
              type="button"
              class="tum-live-button tum-live-button-secondary"
              :disabled="working"
              data-testid="cut-abandon"
              @click="confirmingAbandon = true"
            >
              Give up the cut
            </button>
          </div>
        </div>

        <p v-if="outcome === 'done'" class="flex flex-wrap gap-2 text-sm">
          <RouterLink :to="watchPath" class="tum-live-button text-white bg-blue-500 dark:bg-indigo-600">
            Watch the cut video
          </RouterLink>
          <RouterLink :to="`/admin/courses/${courseId}/lectures`" class="tum-live-button tum-live-button-secondary">
            Back to the lectures
          </RouterLink>
        </p>
      </section>

      <template v-else-if="phase === 'failed'">
        <RouterLink :to="`/admin/courses/${courseId}/lectures`" class="tum-live-button tum-live-button-secondary w-fit">
          Back to the lectures
        </RouterLink>
      </template>

      <template v-else>
        <VideoPlayer
          ref="player"
          :src="src"
          hotkeys="player"
          :skip-seconds="10"
          @timeupdate="onTime"
          data-testid="cut-player"
        />

        <p v-if="phase === 'building'" class="text-5 flex items-center gap-2 text-sm" role="status">
          <i class="fas fa-circle-notch fa-spin"></i>
          Building the audio timeline. This takes a minute or two for a long lecture.
        </p>

        <template v-else-if="waveform">
          <p
            v-if="versionsDiffer"
            class="rounded-lg bg-danger/25 px-2 py-2 text-sm"
            role="alert"
            data-testid="cut-versions-differ"
          >
            The versions of this recording (<span v-for="(d, v) in waveform.versionDurations" :key="v"
              >{{ v }} {{ formatTime(d) }}; </span
            >) differ in length by more than 30 seconds, so they can't be cut together.
          </p>

          <div class="flex flex-wrap items-center gap-2">
            <button
              v-if="pendingStart === null"
              type="button"
              class="tum-live-button text-white bg-blue-500 dark:bg-indigo-600"
              data-testid="cut-start"
              @click="markStart"
            >
              <i class="fas fa-scissors mr-1"></i>Start cut here ({{ formatTime(currentTime, true) }})
            </button>
            <template v-else>
              <button
                type="button"
                class="tum-live-button text-white bg-red-600"
                data-testid="cut-end"
                @click="markEnd"
              >
                <i class="fas fa-scissors mr-1"></i>End cut here ({{ formatTime(currentTime, true) }})
              </button>
              <button type="button" class="tum-live-button tum-live-button-secondary" @click="pendingStart = null">
                Cancel
              </button>
              <span class="text-5 text-xs">from {{ formatTime(pendingStart, true) }}</span>
            </template>

            <label class="text-3 ml-auto flex items-center gap-1 text-sm">
              <input v-model="skipCuts" type="checkbox" />
              Preview without the cuts
            </label>
            <div class="flex items-center gap-1" role="group" aria-label="Zoom">
              <button
                type="button"
                class="tum-live-button tum-live-button-secondary"
                :disabled="zoom <= 1"
                aria-label="Zoom out"
                @click="zoomOut"
              >
                <i class="fas fa-magnifying-glass-minus"></i>
              </button>
              <span class="text-5 w-8 text-center text-xs">×{{ zoom }}</span>
              <button
                type="button"
                class="tum-live-button tum-live-button-secondary"
                :disabled="zoom >= 16"
                aria-label="Zoom in"
                @click="zoomIn"
              >
                <i class="fas fa-magnifying-glass-plus"></i>
              </button>
            </div>
          </div>

          <CutTimeline
            :peaks="waveform.peaks"
            :bucket-seconds="waveform.bucketSeconds"
            :duration="duration"
            :cuts="cuts"
            :current-time="currentTime"
            :pending-start="pendingStart"
            :zoom="zoom"
            :selected-id="selectedId"
            @seek="seek"
            @create="create"
            @resize="resize"
            @select="selectedId = $event"
            @remove="remove"
          />

          <section v-if="suggestions.length" class="rounded-lg border border-amber-500/60 p-3" data-testid="cut-suggestions">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="text-1 text-sm font-semibold">
                {{ suggestions.length }} suggested {{ suggestions.length === 1 ? "cut" : "cuts" }} from the silences in the recording
              </h2>
              <button
                type="button"
                class="tum-live-button tum-live-button-secondary ml-auto"
                data-testid="cut-accept-all"
                @click="cuts = acceptAll(cuts, duration)"
              >
                Accept all
              </button>
              <button type="button" class="tum-live-button tum-live-button-secondary" @click="dismissSuggestions">
                Dismiss all
              </button>
            </div>
            <p class="text-5 text-xs">Suggestions are not cut until you accept them.</p>
          </section>

          <section aria-labelledby="cut-list-heading">
            <h2 id="cut-list-heading" class="text-5 border-b text-xs font-semibold uppercase tracking-wide dark:border-gray-700">
              Cuts
            </h2>
            <p v-if="!cuts.length" class="text-5 py-2 text-sm">
              Drag over the timeline, or mark a start and an end while the video plays.
            </p>
            <ul v-else class="divide-y text-sm dark:divide-gray-700">
              <li
                v-for="c in cuts"
                :key="c.id"
                class="flex flex-wrap items-center gap-2 py-1"
                :class="selectedId === c.id ? 'bg-blue-500/10' : ''"
                :data-testid="c.suggested ? 'cut-row-suggested' : 'cut-row'"
              >
                <button type="button" class="text-2 font-mono hover:underline" @click="seek(c.start)">
                  {{ formatTime(c.start, true) }} – {{ formatTime(c.end, true) }}
                </button>
                <span class="text-5 text-xs">{{ formatTime(c.end - c.start) }}</span>
                <span v-if="c.suggested" class="rounded bg-amber-400/30 px-1 text-xs">suggested</span>
                <span class="ml-auto flex gap-1">
                  <button
                    v-if="c.suggested"
                    type="button"
                    class="tum-live-button tum-live-button-secondary"
                    @click="cuts = acceptCut(cuts, c.id, duration)"
                  >
                    Accept
                  </button>
                  <button type="button" class="tum-live-button tum-live-button-secondary" @click="remove(c.id)">
                    {{ c.suggested ? "Dismiss" : "Remove" }}
                  </button>
                </span>
              </li>
            </ul>
          </section>

          <section class="flex flex-col gap-2 border-t pt-3 dark:border-gray-700">
            <p class="text-sm" data-testid="cut-summary">
              <template v-if="applied.length">
                Removing {{ formatTime(removed) }} of {{ formatTime(duration) }}. The recording will be
                {{ formatTime(kept) }} long.
              </template>
              <template v-else>Nothing is cut yet.</template>
            </p>
            <p v-if="applied.length && !leavesEnough(cuts, duration)" class="text-sm text-red-700 dark:text-red-400" role="alert">
              The cuts must leave at least a second of the recording.
            </p>

            <fieldset class="flex flex-col gap-1 text-sm">
              <legend class="text-2 mb-1">When the cut is done</legend>
              <label class="flex items-start gap-2">
                <input v-model="mode" type="radio" :value="LectureCutMode.OVERWRITE" name="cut-mode" class="mt-1" />
                <span><strong>Replace the recording.</strong> The original is gone afterwards.</span>
              </label>
              <label class="flex items-start gap-2">
                <input v-model="mode" type="radio" :value="LectureCutMode.DUPLICATE" name="cut-mode" class="mt-1" />
                <span>
                  <strong>Make a new lecture.</strong> It is visible to the same people as this one, and this one
                  becomes private.
                </span>
              </label>
            </fieldset>

            <div v-if="confirming" class="flex flex-wrap items-center gap-2 rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alertdialog">
              <span>This replaces the recording of every version and can't be undone.</span>
              <button type="button" class="tum-live-button text-white bg-red-600" data-testid="cut-confirm" @click="submit">
                Replace it
              </button>
              <button type="button" class="tum-live-button tum-live-button-secondary" @click="confirming = false">
                Cancel
              </button>
            </div>
            <div v-else>
              <button
                type="button"
                class="tum-live-button text-white bg-blue-500 disabled:opacity-50 dark:bg-indigo-600"
                :disabled="!canSubmit"
                data-testid="cut-submit"
                @click="submitClicked"
              >
                <i v-if="submitting" class="fas fa-circle-notch fa-spin mr-1"></i>
                <i v-else class="fas fa-scissors mr-1"></i>Cut video
              </button>
            </div>
          </section>
        </template>
      </template>
    </section>
  </AdminLayout>
</template>
