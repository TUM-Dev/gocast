<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { onBeforeRouteLeave } from "vue-router";

import type { CourseLecture } from "@/lib/course-lectures";
import { buildCreatePlan, createErrorMessage, createLectures, type CreateLectureForm } from "@/lib/create-lecture";
import { opfsSupported, type RecordingVersion } from "@/lib/recorder/chunk-store";
import { saveLocalCopy } from "@/lib/recorder/recovery";
import { LOW_STORAGE_BYTES, freeStorage } from "@/lib/recorder/session";
import { useBrowserRecordingStore } from "@/stores/browser-recording";

/**
 * The wizard's last step for a recording: share the screen and turn on the camera,
 * record, stop. The lecture is created when recording starts, as its date is then.
 *
 * What is recorded goes to disk and is uploaded while recording, so a network outage
 * or a crashed tab costs at most a few seconds; see lib/recorder.
 */
const props = defineProps<{ courseId: number; form: CreateLectureForm }>();

const emit = defineEmits<{
  /** The lecture, once its recording is finalized; `problem` when something went wrong. */
  created: [lectures: CourseLecture[], problem: string];
}>();

const rec = useBrowserRecordingStore();
const supported = ref(
  opfsSupported() && typeof MediaRecorder !== "undefined" && !!navigator.mediaDevices?.getDisplayMedia,
);

const screen = ref<MediaStream | null>(null);
const camera = ref<MediaStream | null>(null);
const screenVideo = ref<HTMLVideoElement | null>(null);
const cameraVideo = ref<HTMLVideoElement | null>(null);
const error = ref("");
const busy = ref("");
const lowStorage = ref(false);
const stopping = ref(false);

watch(screen, (s) => screenVideo.value && (screenVideo.value.srcObject = s));
watch(camera, (s) => cameraVideo.value && (cameraVideo.value.srcObject = s));

onMounted(async () => {
  if (supported.value && !rec.session) supported.value = await rec.storageWorks();
  const free = await freeStorage();
  lowStorage.value = free !== null && free < LOW_STORAGE_BYTES;
});

async function shareScreen(): Promise<MediaStream | null> {
  error.value = "";
  try {
    return await navigator.mediaDevices.getDisplayMedia({ video: { frameRate: { ideal: 15 } }, audio: false });
  } catch {
    error.value = "The screen was not shared.";
    return null;
  }
}

async function turnOnMicrophone(): Promise<MediaStreamTrack | null> {
  error.value = "";
  try {
    return (await navigator.mediaDevices.getUserMedia({ audio: true })).getAudioTracks()[0] ?? null;
  } catch {
    error.value = "The microphone could not be turned on. Check the browser's permissions.";
    return null;
  }
}

async function turnOnCamera(): Promise<MediaStream | null> {
  error.value = "";
  try {
    return await navigator.mediaDevices.getUserMedia({ video: { width: { ideal: 1280 } }, audio: true });
  } catch {
    error.value = "The camera and microphone could not be turned on. Check the browser's permissions.";
    return null;
  }
}

async function chooseScreen(): Promise<void> {
  screen.value?.getTracks().forEach((t) => t.stop());
  screen.value = await shareScreen();
}

async function chooseCamera(): Promise<void> {
  camera.value?.getTracks().forEach((t) => t.stop());
  camera.value = await turnOnCamera();
}

const canStart = computed(() => !!screen.value || !!camera.value);

async function start(): Promise<void> {
  error.value = "";
  // Before anything is awaited: the audio has to start on the click.
  const session = rec.begin(props.courseId);
  session.prepareAudio();
  const giveUp = async () => {
    await session.discard();
    rec.clear();
    busy.value = "";
  };

  // The camera's microphone, or one of its own: a screen alone records no sound.
  const mic = camera.value?.getAudioTracks()[0] ?? (await turnOnMicrophone());
  if (!mic && !window.confirm("Record without sound?")) return giveUp();

  busy.value = "Creating the lecture…";
  let lecture: CourseLecture | undefined;
  try {
    const plan = buildCreatePlan(props.courseId, props.form, new Date());
    lecture = (await createLectures(props.courseId, plan.request))[0];
  } catch (err) {
    error.value = createErrorMessage(err);
    return giveUp();
  }
  if (!lecture) return giveUp();
  rec.setLecture(lecture);

  busy.value = "Starting the recording…";
  try {
    await session.start(props.courseId, lecture.id, props.form.title.trim(), {
      screen: screen.value,
      camera: camera.value,
      mic,
    });
  } catch (err) {
    error.value = `The recording could not be started: ${err instanceof Error ? err.message : err}. The lecture was created; upload a video to it from its card.`;
    return giveUp();
  }
  busy.value = "";
}

/* Recording. */

const now = ref(Date.now());
const timer = setInterval(() => (now.value = Date.now()), 1000);
onBeforeUnmount(() => clearInterval(timer));

const elapsed = computed(() => {
  const start = rec.session?.startedAt;
  if (!start) return "";
  const s = Math.max(0, Math.floor((now.value - start) / 1000));
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${Math.floor(s / 3600)}:${pad(Math.floor(s / 60) % 60)}:${pad(s % 60)}`;
});

const LABEL: Record<RecordingVersion, string> = { PRES: "Screen", CAM: "Camera" };

const paused = computed(() => rec.versions.some((v) => v.state === "paused"));
/** Something is being recorded or paused, which Pause and Resume act on. */
const pausable = computed(() => rec.versions.some((v) => v.state === "recording" || v.state === "paused"));
const pendingBytes = computed(() => rec.versions.reduce((n, v) => n + v.upload.pendingBytes, 0));
const unreachable = computed(() => rec.versions.some((v) => v.upload.state === "retrying"));
const allDone = computed(() => rec.versions.length > 0 && rec.versions.every((v) => v.upload.state === "done"));
const missing = computed(() => rec.versions.some((v) => v.upload.missing));
const empty = computed(() => rec.versions.every((v) => v.upload.empty));

function megabytes(bytes: number): string {
  return `${(bytes / 1024 ** 2).toFixed(bytes < 10 * 1024 ** 2 ? 1 : 0)} MB`;
}

async function shareAgain(version: RecordingVersion): Promise<void> {
  const stream = version === "PRES" ? await shareScreen() : await turnOnCamera();
  const video = stream?.getVideoTracks()[0];
  if (!stream || !video) return;
  if (version === "PRES") screen.value = stream;
  else camera.value = stream;
  // A camera turned on again brings a microphone, which takes over if the one recording went.
  const audio = stream.getAudioTracks()[0];
  if (audio && rec.micState !== "on") rec.session?.replaceMic(audio);
  else audio?.stop();
  rec.session?.resumeWith(version, video);
}

async function micAgain(): Promise<void> {
  const track = await turnOnMicrophone();
  if (track) rec.session?.replaceMic(track);
}

async function stop(): Promise<void> {
  if (!rec.session) return;
  stopping.value = true;
  await rec.session.stop();
  stopping.value = false;
  screen.value = camera.value = null;
}

/* Immediate: the upload may have finished while this was not mounted. */
watch(
  allDone,
  (done) => {
    const lecture = rec.lecture;
    const ownCourse = rec.courseId === props.courseId;
    if (!done || !lecture) return;
    let problem = "";
    if (empty.value) problem = "Nothing was recorded. Upload a video to the lecture from its card, or delete it.";
    else if (missing.value) problem = "The recording was finished, but parts of it never reached the server and are missing.";
    rec.clear();
    // A lecture of another course has no place in this one's list.
    if (ownCourse) emit("created", [lecture], problem);
  },
  { immediate: true },
);

async function discard(): Promise<void> {
  if (!rec.session) return;
  if (!window.confirm("Throw the recording away? This cannot be undone. The lecture itself stays.")) return;
  const lecture = rec.lecture;
  const ownCourse = rec.courseId === props.courseId;
  const result = await rec.session.discard();
  rec.clear();
  screen.value = camera.value = null;
  const message = {
    gone: "The recording was thrown away.",
    pending:
      "The server could not be reached to throw the recording away. It is kept in this browser and thrown away once the server is back, the next time this course's lectures are opened.",
    refused: "The recording could not be thrown away: it is being converted already. Delete the version from the lecture's card.",
  }[result];
  if (lecture && ownCourse) emit("created", [lecture], message);
}

async function saveCopy(version: RecordingVersion): Promise<void> {
  const meta = rec.session?.metas.find((m) => m.version === version);
  if (meta) await saveLocalCopy(rec.chunks, meta);
}

/* Leaving. The recording lives in the store, not here, so leaving the page stops it
 * deliberately rather than leaving it running out of sight. The upload goes on. */
onBeforeRouteLeave(() => {
  if (!rec.recording) return true;
  if (!window.confirm("Leaving this page stops the recording. What was recorded is uploaded in the background.")) {
    return false;
  }
  void rec.session?.stop();
  return true;
});

onBeforeUnmount(() => {
  if (!rec.session) {
    screen.value?.getTracks().forEach((t) => t.stop());
    camera.value?.getTracks().forEach((t) => t.stop());
  }
});
</script>

<template>
  <div class="flex flex-col gap-4">
    <p v-if="!supported" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
      This browser cannot record here. Use a recent Chrome, Edge, Firefox or Safari on a computer.
    </p>

    <template v-else>
      <template v-if="!rec.session">
      <p class="text-5 text-sm">
        Share the screen for the presentation video and turn on the camera for the camera video; either is enough. The
        microphone goes into both. What is recorded is kept in this browser and uploaded while you record.
      </p>
      <p v-if="lowStorage" class="rounded-lg bg-warn/25 px-2 py-2 text-sm" role="alert">
        This browser has little storage left for this site. If the connection drops for long, the recording may not
        fit.
      </p>

      </template>

      <!-- Shown while recording too, so the lecturer sees what is recorded. -->
      <div class="grid gap-3 sm:grid-cols-2">
        <div class="flex flex-col gap-2">
          <video
            v-show="screen"
            ref="screenVideo"
            class="aspect-video w-full rounded bg-black"
            autoplay
            muted
            playsinline
          ></video>
          <button
            v-if="!rec.session"
            type="button"
            class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm"
            @click="chooseScreen"
          >
            <i class="fas fa-display mr-1"></i>{{ screen ? "Share another screen" : "Share screen" }}
          </button>
        </div>
        <div class="flex flex-col gap-2">
          <video
            v-show="camera"
            ref="cameraVideo"
            class="aspect-video w-full rounded bg-black"
            autoplay
            muted
            playsinline
          ></video>
          <button
            v-if="!rec.session"
            type="button"
            class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm"
            @click="chooseCamera"
          >
            <i class="fas fa-video mr-1"></i>{{ camera ? "Choose another camera" : "Turn on camera" }}
          </button>
        </div>
      </div>

      <button
        v-if="!rec.session"
        type="button"
        class="tum-live-button-primary self-start px-3 py-1 text-sm"
        :disabled="!canStart || !!busy"
        @click="start"
      >
        <span class="mr-1 inline-block h-2.5 w-2.5 rounded-full bg-red-500" aria-hidden="true"></span>Start recording
      </button>

      <template v-if="rec.session">
        <div class="flex flex-wrap items-center gap-3">
          <span v-if="rec.recording" class="flex items-center gap-2 font-semibold" role="status">
            <span
              class="h-3 w-3 rounded-full"
              :class="paused || !pausable ? 'bg-gray-400' : 'animate-pulse bg-red-600'"
            ></span>
            {{ !pausable ? "Not recording" : paused ? "Paused" : "Recording" }} {{ elapsed }}
          </span>
          <span v-else-if="!allDone" class="text-3 flex items-center gap-2 text-sm" role="status">
            <i class="fas fa-circle-notch animate-spin"></i>Finishing the upload…
          </span>
        </div>

        <ul class="flex flex-col gap-1 text-sm" aria-label="Versions">
          <li v-for="v in rec.versions" :key="v.version" class="flex flex-wrap items-center gap-2">
            <span class="text-2 w-16 font-semibold">{{ LABEL[v.version] }}</span>
            <span v-if="v.state === 'waiting'" class="text-amber-600 dark:text-amber-400">
              Not recording; until it is shared again, this video shows black.
            </span>
            <button
              v-if="v.state === 'waiting'"
              type="button"
              class="tum-live-button-secondary tum-live-button px-2 py-0.5 text-xs"
              @click="shareAgain(v.version)"
            >
              {{ v.version === "PRES" ? "Share screen again" : "Turn camera on again" }}
            </button>
            <span v-if="v.upload.state === 'done'" class="text-5">Uploaded; converting.</span>
            <span v-else-if="v.upload.state === 'failed'" class="text-red-600 dark:text-red-400">{{ v.upload.error }}</span>
            <button
              v-if="v.upload.state === 'failed'"
              type="button"
              class="tum-live-button-secondary tum-live-button px-2 py-0.5 text-xs"
              @click="saveCopy(v.version)"
            >
              Save a copy
            </button>
          </li>
        </ul>

        <p
          v-if="rec.recording && rec.micState === 'lost'"
          class="flex flex-wrap items-center gap-2 rounded-lg bg-warn/25 px-2 py-2 text-sm"
          role="alert"
        >
          The microphone stopped: the recording goes on without sound.
          <button type="button" class="tum-live-button-secondary tum-live-button px-2 py-0.5 text-xs" @click="micAgain">
            Turn microphone on again
          </button>
        </p>
        <p v-if="unreachable" class="rounded-lg bg-warn/25 px-2 py-2 text-sm" role="alert">
          The server can't be reached. Recording goes on and is kept in this browser
          ({{ megabytes(pendingBytes) }} waiting); it is uploaded once the server is back.
          <strong>Do not reload or close this page</strong> until then.
        </p>
        <p v-else-if="pendingBytes > 4 * 1024 ** 2" class="text-5 text-sm">{{ megabytes(pendingBytes) }} waiting to upload.</p>
        <p v-if="rec.storageError" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
          Part of the recording could not be stored in this browser: {{ rec.storageError }}
        </p>

        <div class="flex flex-wrap gap-2">
          <template v-if="rec.recording">
            <button
              v-if="pausable"
              type="button"
              class="tum-live-button-secondary tum-live-button px-3 py-1 text-sm"
              @click="paused ? rec.session.resume() : rec.session.pause()"
            >
              <i class="fas mr-1" :class="paused ? 'fa-play' : 'fa-pause'"></i>{{ paused ? "Resume" : "Pause" }}
            </button>
            <button type="button" class="tum-live-button-primary px-3 py-1 text-sm" :disabled="stopping" @click="stop">
              <i class="fas fa-stop mr-1"></i>Stop and upload
            </button>
          </template>
          <button
            v-if="!allDone"
            type="button"
            class="tum-live-button-secondary tum-live-button ml-auto px-3 py-1 text-sm"
            @click="discard"
          >
            Throw away
          </button>
        </div>
      </template>
    </template>

    <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>
    <p v-if="busy" class="text-3 flex items-center gap-2 text-sm" role="status">
      <i class="fas fa-circle-notch animate-spin"></i>{{ busy }}
    </p>
  </div>
</template>
