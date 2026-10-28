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

// Options render as pills, but the native input stays on top of each one, transparent,
// so clicks, keyboard and the e2e tests' getByRole("radio") all reach a real input.
// The label text ignores the pointer so it never covers the input.
// The chosen one is filled in high-contrast grey rather than a colour, so a card full
// of option groups stays calm; blue is kept for the buttons that act.
const pillClass =
  "text-4 relative flex cursor-pointer items-center rounded-full px-3 py-1 text-sm font-medium ring-1 ring-gray-300 transition-colors not-has-checked:hover:bg-gray-200/60 has-checked:bg-gray-800 has-checked:text-white has-checked:ring-gray-800 has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-blue-500 has-disabled:cursor-not-allowed has-disabled:opacity-60 dark:ring-gray-600 dark:not-has-checked:hover:bg-gray-700/60 dark:has-checked:bg-gray-100 dark:has-checked:text-gray-900 dark:has-checked:ring-gray-100 dark:has-focus-visible:outline-indigo-600";
const pillInputClass = "absolute inset-0 cursor-pointer appearance-none rounded-full disabled:cursor-not-allowed";
const rowClass = "flex flex-col gap-3 py-4 first:pt-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between";
const cardClass = "rounded-2xl bg-gray-50 p-5 dark:bg-secondary-light";
const iconClass =
  "flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-300";
/** How long "… saved." stays up. Errors stay until dismissed or the next save. */
const STATUS_MS = 3000;
let statusTimer: ReturnType<typeof setTimeout> | undefined;

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

/** Saves one setting and reports the outcome in the toast. */
async function save(type: UserSettingType, value: unknown, note: string): Promise<void> {
  error.value = "";
  status.value = "";
  try {
    await updateSetting(type, value);
    status.value = note;
    clearTimeout(statusTimer);
    statusTimer = setTimeout(() => (status.value = ""), STATUS_MS);
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
  <section class="text-3 mx-auto flex w-full max-w-3xl flex-col gap-6 p-4 md:py-8">
    <header class="flex items-start justify-between gap-4">
      <div>
        <h1 class="text-1 text-2xl font-bold">Settings</h1>
        <p class="text-5 mt-1 text-sm">How TUM-Live greets you and plays your lectures.</p>
      </div>
      <!--
        Matches the close-button partial (web/template/partial/close-btn.gohtml).
        The legacy glyph comes from the TUM-Live icon webfont, which the SPA does not
        load; an inline cross renders the same without dragging the font in.
      -->
      <button
        type="button"
        class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-gray-400 transition-colors duration-200 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-secondary-light dark:hover:text-white"
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
    </header>

    <!-- Fixed, so a message coming and going never shifts the page under the cursor. -->
    <div class="pointer-events-none fixed inset-x-0 bottom-6 z-50 flex justify-center px-4">
      <Transition
        mode="out-in"
        enter-active-class="transition duration-200"
        enter-from-class="translate-y-2 opacity-0"
        leave-active-class="transition duration-150"
        leave-to-class="opacity-0"
      >
        <p
          v-if="error"
          class="pointer-events-auto flex items-center gap-2 rounded-full bg-gray-800 py-2 pl-4 pr-2 text-sm text-white shadow-lg dark:bg-gray-100 dark:text-gray-900"
          role="alert"
        >
          <i class="fas fa-circle-exclamation text-red-400 dark:text-red-600" aria-hidden="true"></i>
          <span>{{ error }}</span>
          <button
            type="button"
            class="flex h-6 w-6 items-center justify-center rounded-full opacity-70 hover:bg-white/10 hover:opacity-100 dark:hover:bg-black/10"
            aria-label="Dismiss"
            @click="error = ''"
          >
            <i class="fas fa-xmark" aria-hidden="true"></i>
          </button>
        </p>
        <p
          v-else-if="status"
          class="flex items-center gap-2 rounded-full bg-gray-800 px-4 py-2 text-sm text-white shadow-lg dark:bg-gray-100 dark:text-gray-900"
          role="status"
        >
          <i class="fas fa-circle-check text-green-400 dark:text-green-600" aria-hidden="true"></i>{{ status }}
        </p>
      </Transition>
    </div>

    <p v-if="!settings" class="text-5 text-sm">Loading your settings…</p>

    <template v-else>
      <div :class="cardClass">
        <div class="mb-4 flex items-center gap-3">
          <span :class="iconClass"><i class="fas fa-user" aria-hidden="true"></i></span>
          <div>
            <h2 class="text-1 font-semibold">Profile</h2>
            <p class="text-5 text-xs">How TUM-Live addresses you.</p>
          </div>
        </div>

        <div class="divide-y divide-gray-200 dark:divide-gray-700">
          <div :class="rowClass">
            <div>
              <label class="text-1 text-sm font-medium" for="displayName">Preferred name</label>
              <p class="text-5 text-xs">You can change this once every three months.</p>
            </div>
            <div class="flex gap-2">
              <input
                id="displayName"
                v-model="settings.preferredName"
                class="tum-live-input h-9 rounded-full sm:w-56"
                type="text"
                @keyup.enter="savePreferredName"
              />
              <button
                type="button"
                class="tum-live-button tum-live-button-primary px-4 disabled:cursor-not-allowed"
                :disabled="settings.preferredName === initialName"
                @click="savePreferredName"
              >
                Save
              </button>
            </div>
          </div>

          <div :class="rowClass">
            <p id="greeting-label" class="text-1 text-sm font-medium">Greeting</p>
            <div class="flex flex-wrap gap-2" role="radiogroup" aria-labelledby="greeting-label">
              <label v-for="greeting in GREETINGS" :key="greeting" :class="pillClass">
                <input
                  v-model="settings.greeting"
                  :class="pillInputClass"
                  type="radio"
                  name="greeting"
                  :value="greeting"
                  @change="save(UserSettingType.GREETING, greeting, 'Greeting saved.')"
                />
                <span class="pointer-events-none relative">{{ greeting }}</span>
              </label>
            </div>
          </div>
        </div>
      </div>

      <div :class="cardClass">
        <div class="mb-4 flex items-center gap-3">
          <span :class="iconClass"><i class="fas fa-circle-play" aria-hidden="true"></i></span>
          <div>
            <h2 class="text-1 font-semibold">Playback</h2>
            <p class="text-5 text-xs">Defaults for the video player.</p>
          </div>
        </div>

        <div class="divide-y divide-gray-200 dark:divide-gray-700">
          <div :class="rowClass">
            <div>
              <p id="speeds-label" class="text-1 text-sm font-medium">Playback speeds</p>
              <p class="text-5 text-xs">Which speeds the player offers.</p>
            </div>
            <div class="flex flex-wrap gap-2" role="group" aria-labelledby="speeds-label">
              <label v-for="entry in settings.playbackSpeeds" :key="entry.speed" :class="pillClass">
                <input
                  :class="pillInputClass"
                  type="checkbox"
                  :checked="entry.enabled"
                  :disabled="entry.speed === 1"
                  @change="togglePlaybackSpeed(entry)"
                />
                <span class="pointer-events-none relative">{{ entry.speed }}</span>
              </label>
            </div>
          </div>

          <div :class="rowClass">
            <div>
              <p class="text-1 text-sm font-medium">Custom speeds</p>
              <p class="text-5 text-xs">Up to three, between 0.25 and 5.</p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <button
                v-for="speed in settings.customSpeeds"
                :key="speed"
                type="button"
                class="tum-live-button tum-live-button-tertiary"
                :title="`Remove ${speed}`"
                @click="removeCustomSpeed(speed)"
              >
                {{ speed }} ✕
              </button>

              <label class="sr-only" for="newSpeed">New custom speed</label>
              <input
                id="newSpeed"
                v-model="newSpeed"
                class="tum-live-input h-9 w-24 rounded-full"
                type="number"
                min="0.25"
                max="5"
                step="0.05"
                placeholder="1.00"
                @keyup.enter="addCustomSpeed"
              />
              <button type="button" class="tum-live-button tum-live-button-primary px-4" @click="addCustomSpeed">
                Add
              </button>
            </div>
          </div>

          <div :class="rowClass">
            <div>
              <p id="seeking-label" class="text-1 text-sm font-medium">Seeking time</p>
              <p class="text-5 text-xs">How far skipping forward or back jumps.</p>
            </div>
            <div class="flex flex-wrap gap-2" role="radiogroup" aria-labelledby="seeking-label">
              <label v-for="seconds in SEEKING_TIMES" :key="seconds" :class="pillClass">
                <input
                  v-model.number="settings.seekingTime"
                  :class="pillInputClass"
                  type="radio"
                  name="seekingTime"
                  :value="seconds"
                  @change="save(UserSettingType.SEEKING_TIME, seconds, 'Seeking time saved.')"
                />
                <span class="pointer-events-none relative">{{ seconds }}s</span>
              </label>
            </div>
          </div>

          <div :class="rowClass">
            <p id="view-label" class="text-1 text-sm font-medium">Preferred lecture view</p>
            <div class="flex flex-wrap gap-2" role="radiogroup" aria-labelledby="view-label">
              <label v-for="view in VIEWS" :key="view" :class="pillClass">
                <input
                  v-model="settings.lectureView"
                  :class="pillInputClass"
                  type="radio"
                  name="lectureView"
                  :value="view"
                  @change="save(UserSettingType.LECTURE_VIEW, view, 'Preferred view saved.')"
                />
                <span class="pointer-events-none relative">{{ view }}</span>
              </label>
            </div>
          </div>

          <label :class="[rowClass, 'cursor-pointer']">
            <span class="text-1 text-sm font-medium">Skip the silence at the start of a lecture</span>
            <!-- A switch drawn on the checkbox itself; the knob is its ::after. -->
            <input
              v-model="settings.autoSkip"
              type="checkbox"
              class="relative h-6 w-11 shrink-0 cursor-pointer appearance-none rounded-full bg-gray-300 transition-colors after:absolute after:top-0.5 after:left-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-transform after:content-[''] checked:bg-blue-500 checked:after:translate-x-5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:bg-gray-600 dark:checked:bg-indigo-600"
              @change="save(UserSettingType.AUTO_SKIP, { enabled: settings.autoSkip }, 'Auto skip saved.')"
            />
          </label>
        </div>
      </div>

      <div :class="cardClass">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex items-center gap-3">
            <span :class="iconClass"><i class="fas fa-shield-halved" aria-hidden="true"></i></span>
            <div>
              <h2 class="text-1 font-semibold">Privacy &amp; data protection</h2>
              <p class="text-5 text-xs">Download everything TUM-Live stores about you.</p>
            </div>
          </div>
          <!--
            A direct link rather than an API call: the browser downloads the response,
            and this endpoint is reached with the session cookie the browser already has.
          -->
          <a
            href="/api/v2/users/export"
            class="tum-live-button tum-live-button-muted inline-flex w-fit items-center px-4 py-2"
            download="tum-live-personal-data.json"
          >
            <i class="fas fa-download mr-2" aria-hidden="true"></i>Export my personal data
          </a>
        </div>
      </div>

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
  </section>
</template>
