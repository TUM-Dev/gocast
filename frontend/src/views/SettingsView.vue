<script setup lang="ts">
import { onMounted, ref } from "vue";

import { ApiError } from "@/lib/api";
import {
  UserSettingType,
  sanitizeSpeed,
  updateSetting,
  type LectureView,
  type PlaybackSpeed,
  type UserSettings,
} from "@/lib/settings";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

const auth = useAuthStore();

const settings = ref<UserSettings | null>(null);
const initialName = ref("");
const newSpeed = ref("");
const error = ref("");
const status = ref("");

const GREETINGS = ["Moin", "Servus"];
// Must stay a subset of validSeekingTimes in model/user.go; anything else is silently
// discarded by GetSeekingTime and falls back to 10.
const SEEKING_TIMES = [5, 10, 30];
const VIEWS: LectureView[] = ["Combined", "Presentation", "Camera", "Split"];

onMounted(async () => {
  try {
    const user = await auth.load();
    if (!user) {
      redirectToLogin();
      return;
    }
    settings.value = user.settings;
    initialName.value = user.settings.preferredName;
  } catch (err) {
    error.value = message(err);
  }
});

/** Mirrors the legacy close button, which simply goes back. */
function close(): void {
  if (window.history.length > 1) {
    window.history.back();
    return;
  }
  window.location.assign("/");
}

function message(err: unknown): string {
  if (err instanceof ApiError) {
    return err.isUnauthenticated ? "Your session expired. Please sign in again." : err.message;
  }
  return "Something went wrong. Please try again.";
}

/** Saves one setting and reports the outcome next to the section that changed. */
async function save(type: UserSettingType, value: unknown, note: string): Promise<void> {
  error.value = "";
  status.value = "";
  try {
    await updateSetting(type, value);
    status.value = note;
  } catch (err) {
    error.value = message(err);
    // The server rejected the write, so the local value no longer reflects what is
    // stored. Reload rather than leaving a stale value on screen.
    try {
      const user = await auth.load(true);
      if (user) {
        settings.value = user.settings;
        initialName.value = user.settings.preferredName;
      }
    } catch {
      // The reload failed too, so the screen keeps the value the user typed. Leave
      // the write error showing — it is the actionable one — and do not let this
      // escape: most callers invoke save() as `void save(...)`, where a rejection
      // surfaces as an unhandled promise rejection rather than as anything a user
      // can see.
    }
  }
}

async function savePreferredName(): Promise<void> {
  if (!settings.value) return;
  const name = settings.value.preferredName.trim();
  if (name === "") {
    error.value = "Preferred name cannot be empty.";
    return;
  }
  await save(UserSettingType.PREFERRED_NAME, name, "Preferred name saved.");
  if (!error.value) {
    initialName.value = name;
  }
}

function togglePlaybackSpeed(entry: PlaybackSpeed): void {
  if (!settings.value) return;
  entry.enabled = !entry.enabled;
  void save(UserSettingType.CUSTOM_PLAYBACK_SPEEDS, settings.value.playbackSpeeds, "Playback speeds saved.");
}

function addCustomSpeed(): void {
  if (!settings.value) return;

  const speed = sanitizeSpeed(newSpeed.value);
  if (speed === null) {
    error.value = "Enter a speed between 0.25 and 5.";
    return;
  }
  if (settings.value.customSpeeds.includes(speed)) {
    error.value = `${speed} is already in the list.`;
    return;
  }
  if (settings.value.customSpeeds.length >= 3) {
    error.value = "You can add at most three custom speeds.";
    return;
  }

  settings.value.customSpeeds = [...settings.value.customSpeeds, speed].sort((a, b) => a - b);
  newSpeed.value = "";
  void save(UserSettingType.USER_DEFINED_SPEEDS, settings.value.customSpeeds, "Custom speeds saved.");
}

function removeCustomSpeed(speed: number): void {
  if (!settings.value) return;
  settings.value.customSpeeds = settings.value.customSpeeds.filter((s) => s !== speed);
  void save(UserSettingType.USER_DEFINED_SPEEDS, settings.value.customSpeeds, "Custom speeds saved.");
}
</script>

<template>
  <article class="tum-live-settings-grid">
    <header>
      <div class="flex flex-row justify-between">
        <h1 class="text-3 font-bold">Settings</h1>
        <!--
          Matches the close-button partial (web/template/partial/close-btn.gohtml).
          The legacy glyph comes from the TUM-Live icon webfont, which the SPA does not
          load; an inline cross renders the same without dragging the font in.
        -->
        <div class="h-fit w-fit">
          <button
            type="button"
            class="w-4 text-xl text-gray-400 transition-colors duration-200 hover:text-gray-600 dark:hover:text-white"
            title="Close"
            @click="close"
          >
            <span class="flex items-center" aria-hidden="true">
              <svg viewBox="0 0 16 16" class="h-4 w-4" fill="none" stroke="currentColor">
                <path d="M3 3l10 10M13 3L3 13" stroke-width="1.5" stroke-linecap="round" />
              </svg>
            </span>
            <span class="sr-only">Close</span>
          </button>
        </div>
      </div>
    </header>

    <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
      {{ error }}
    </p>
    <p v-else-if="status" class="text-5 text-sm" role="status">{{ status }}</p>

    <p v-if="!settings" class="text-5 text-sm">Loading your settings…</p>

    <template v-else>
      <section>
        <h2>
          Preferred Name
          <span class="pl-2 font-bold italic">You can change this once every three months.</span>
        </h2>
        <div class="flex gap-2">
          <label class="sr-only" for="displayName">Preferred name</label>
          <input
            id="displayName"
            v-model="settings.preferredName"
            class="tum-live-input"
            type="text"
            @keyup.enter="savePreferredName"
          />
          <button
            type="button"
            class="tum-live-input-submit tum-live-button-primary px-4 py-2 text-sm"
            :disabled="settings.preferredName === initialName"
            @click="savePreferredName"
          >
            Save
          </button>
        </div>
      </section>

      <section>
        <h2>Preferred greeting</h2>
        <div class="flex gap-4">
          <label v-for="greeting in GREETINGS" :key="greeting" class="flex items-center gap-2">
            <input
              v-model="settings.greeting"
              class="w-auto"
              type="radio"
              name="greeting"
              :value="greeting"
              @change="save(UserSettingType.GREETING, greeting, 'Greeting saved.')"
            />
            {{ greeting }}
          </label>
        </div>
      </section>

      <section>
        <h2>Playback Speeds</h2>
        <div class="flex flex-wrap gap-4">
          <label
            v-for="entry in settings.playbackSpeeds"
            :key="entry.speed"
            class="flex items-center gap-2"
            :class="{ 'opacity-50': entry.speed === 1 }"
          >
            <input
              class="w-auto"
              type="checkbox"
              :checked="entry.enabled"
              :disabled="entry.speed === 1"
              @change="togglePlaybackSpeed(entry)"
            />
            {{ entry.speed }}
          </label>
        </div>
      </section>

      <section>
        <h2>Custom Speeds (up to 3)</h2>
        <div class="flex flex-wrap items-center gap-2">
          <button
            v-for="speed in settings.customSpeeds"
            :key="speed"
            type="button"
            class="tum-live-input-submit tum-live-button-tertiary px-2 py-1 text-sm"
            :title="`Remove ${speed}`"
            @click="removeCustomSpeed(speed)"
          >
            {{ speed }} ✕
          </button>

          <label class="sr-only" for="newSpeed">New custom speed</label>
          <input
            id="newSpeed"
            v-model="newSpeed"
            class="tum-live-input w-24"
            type="number"
            min="0.25"
            max="5"
            step="0.05"
            placeholder="1.00"
            @keyup.enter="addCustomSpeed"
          />
          <button
            type="button"
            class="tum-live-input-submit tum-live-button-primary px-3 py-2 text-sm"
            @click="addCustomSpeed"
          >
            Add
          </button>
        </div>
      </section>

      <section>
        <h2>Seeking Time in Seconds</h2>
        <div class="flex gap-4">
          <label v-for="seconds in SEEKING_TIMES" :key="seconds" class="flex items-center gap-2">
            <input
              v-model.number="settings.seekingTime"
              class="w-auto"
              type="radio"
              name="seekingTime"
              :value="seconds"
              @change="save(UserSettingType.SEEKING_TIME, seconds, 'Seeking time saved.')"
            />
            {{ seconds }}s
          </label>
        </div>
      </section>

      <section>
        <h2>Automatically Skip First Silence</h2>
        <label class="flex items-center gap-2">
          <input
            v-model="settings.autoSkip"
            class="w-auto"
            type="checkbox"
            @change="save(UserSettingType.AUTO_SKIP, { enabled: settings.autoSkip }, 'Auto skip saved.')"
          />
          Skip the silence at the start of a lecture
        </label>
      </section>

      <section>
        <h2>Preferred Lecture View</h2>
        <div class="flex flex-wrap gap-4">
          <label v-for="view in VIEWS" :key="view" class="flex items-center gap-2">
            <input
              v-model="settings.lectureView"
              class="w-auto"
              type="radio"
              name="lectureView"
              :value="view"
              @change="save(UserSettingType.LECTURE_VIEW, view, 'Preferred view saved.')"
            />
            {{ view }}
          </label>
        </div>
      </section>

      <section>
        <h2>Privacy &amp; Data Protection</h2>
        <!--
          A direct link rather than an API call: the browser downloads the response,
          and this endpoint is reached with the session cookie the browser already has.
        -->
        <a
          href="/api/v2/users/export"
          class="tum-live-input-submit tum-live-button-muted block py-2 text-center text-sm"
          download="tum-live-personal-data.json"
        >
          Export my personal data
        </a>
      </section>

      <footer class="text-5 text-center text-sm">
        <i>
          Not a lot going on here <b>yet</b>.
          <a
            class="underline"
            target="_blank"
            rel="noopener"
            href="https://github.com/TUM-Dev/gocast/issues/new/choose"
            >Open an issue</a
          >
          if you have any ideas what settings you miss :)
        </i>
      </footer>
    </template>
  </article>
</template>
