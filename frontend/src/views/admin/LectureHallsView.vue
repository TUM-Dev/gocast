<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  StreamProtocol,
  deleteLectureHall,
  fetchLectureHalls,
  refreshLectureHallPresets,
  setDefaultCameraPreset,
  takeCameraPresetSnapshot,
  updateLectureHall,
  type CameraPreset,
  type LectureHall,
  type LectureHallInput,
} from "@/lib/lecture-halls";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Lecture hall administration: list, inline edit, delete and camera preset
 * management. Creating a new hall is its own route/page (LectureHallCreateView),
 * same as the page this replaces.
 */
const auth = useAuthStore();

const halls = ref<LectureHall[]>([]);
const loading = ref(true);
const error = ref("");
const filter = ref("");

/**
 * Editable copy of a hall's fields, plus the row's own save state and the state of
 * its camera preset actions. Presets read straight off row.hall -- they are not part
 * of the edit form, so there is nothing to diff or reset.
 */
interface Row {
  hall: LectureHall;
  form: LectureHallInput;
  saving: boolean;
  saved: boolean;
  error: string;
  refreshingPresets: boolean;
  presetsError: string;
  /** presetId of the snapshot currently in flight, so only that tile shows busy. */
  snapshotting: number | null;
}

function toForm(hall: LectureHall): LectureHallInput {
  return {
    name: hall.name,
    streamProtocol: hall.streamProtocol,
    combIp: hall.combIp,
    presIp: hall.presIp,
    camIp: hall.camIp,
    cameraIp: hall.cameraIp,
    pwrCtrlIp: hall.pwrCtrlIp,
  };
}

function toRow(hall: LectureHall): Row {
  return reactive({
    hall,
    form: toForm(hall),
    saving: false,
    saved: false,
    error: "",
    refreshingPresets: false,
    presetsError: "",
    snapshotting: null,
  });
}

const rows = ref<Row[]>([]);

const filteredRows = computed(() => {
  const needle = filter.value.trim().toLowerCase();
  if (!needle) return rows.value;
  return rows.value.filter((row) => row.hall.name.toLowerCase().includes(needle));
});

function changed(row: Row): boolean {
  const f = row.form;
  const h = row.hall;
  return (
    f.name !== h.name ||
    f.streamProtocol !== h.streamProtocol ||
    f.combIp !== h.combIp ||
    f.presIp !== h.presIp ||
    f.camIp !== h.camIp ||
    f.cameraIp !== h.cameraIp ||
    f.pwrCtrlIp !== h.pwrCtrlIp
  );
}

function resetRow(row: Row): void {
  row.form = toForm(row.hall);
  row.error = "";
}

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to administer lecture halls.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function load(): Promise<void> {
  try {
    halls.value = await fetchLectureHalls();
    rows.value = halls.value.map(toRow);
    error.value = "";
  } catch (err) {
    error.value = message(err);
  } finally {
    loading.value = false;
  }
}

onMounted(async () => {
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
    return;
  }
  await load();
});

async function save(row: Row): Promise<void> {
  row.error = "";
  row.saved = false;
  row.saving = true;
  try {
    const updated = await updateLectureHall(row.hall.id, {
      ...row.form,
      name: row.form.name.trim(),
      combIp: row.form.combIp.trim(),
      presIp: row.form.presIp.trim(),
      camIp: row.form.camIp.trim(),
      cameraIp: row.form.cameraIp.trim(),
      pwrCtrlIp: row.form.pwrCtrlIp.trim(),
    });
    row.hall = updated;
    row.form = toForm(updated);
    row.saved = true;
    setTimeout(() => {
      row.saved = false;
    }, 3000);
  } catch (err) {
    row.error = message(err);
  } finally {
    row.saving = false;
  }
}

async function remove(row: Row): Promise<void> {
  if (!window.confirm(`Do you really want to remove "${row.hall.name}"?`)) return;

  try {
    await deleteLectureHall(row.hall.id);
    rows.value = rows.value.filter((r) => r !== row);
    halls.value = halls.value.filter((h) => h.id !== row.hall.id);
  } catch (err) {
    row.error = message(err);
  }
}

/** /public/<image>, falling back to the placeholder for a preset never snapshotted. */
function presetImageUrl(preset: CameraPreset): string {
  return `/public/${preset.image || "noPreset.jpg"}`;
}

async function reloadPresets(row: Row): Promise<void> {
  row.presetsError = "";
  row.refreshingPresets = true;
  try {
    const updated = await refreshLectureHallPresets(row.hall.id);
    row.hall = updated;
  } catch (err) {
    row.presetsError = message(err);
  } finally {
    row.refreshingPresets = false;
  }
}

async function makeDefault(row: Row, preset: CameraPreset): Promise<void> {
  row.presetsError = "";
  try {
    await setDefaultCameraPreset(preset.lectureHallId, preset.presetId);
    row.hall.cameraPresets = row.hall.cameraPresets.map((p) => ({
      ...p,
      isDefault: p.presetId === preset.presetId,
    }));
  } catch (err) {
    row.presetsError = message(err);
  }
}

async function snapshotPreset(row: Row, preset: CameraPreset): Promise<void> {
  row.presetsError = "";
  row.snapshotting = preset.presetId;
  try {
    const updated = await takeCameraPresetSnapshot(preset.lectureHallId, preset.presetId);
    row.hall.cameraPresets = row.hall.cameraPresets.map((p) =>
      p.presetId === preset.presetId ? updated : p,
    );
  } catch (err) {
    row.presetsError = message(err);
  } finally {
    row.snapshotting = null;
  }
}
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex w-full max-w-4xl flex-col gap-6">
      <div class="flex flex-wrap items-center gap-3">
        <h1 class="text-1 mr-auto text-2xl font-bold">
          Lecture Halls <span class="text-5 text-base font-normal">({{ halls.length }})</span>
        </h1>
        <input
          v-model="filter"
          type="search"
          placeholder="Filter by name"
          aria-label="Filter lecture halls by name"
          class="tum-live-input w-64"
        />
        <RouterLink to="/admin/lecture-halls/new" class="tum-live-button-primary px-4 py-2 text-sm">
          + New Lecture Hall
        </RouterLink>
      </div>

      <Transition name="fade" mode="out-in">
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
          {{ error }}
        </p>
      </Transition>

      <p v-if="loading" class="text-5 text-sm">Loading lecture halls…</p>
      <p v-else-if="!halls.length" class="text-5 text-sm">
        No lecture halls yet. Create one to assign lectures to it.
      </p>
      <p v-else-if="!filteredRows.length" class="text-5 text-sm">
        No lecture hall matches "{{ filter }}".
      </p>

      <form
        v-for="row in filteredRows"
        :key="row.hall.id"
        class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800"
        @submit.prevent="save(row)"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-1 mr-auto font-semibold">{{ row.hall.name }}</span>
          <span
            v-if="changed(row)"
            class="flex items-center rounded-full border border-amber-200 bg-amber-200/50 px-2 py-1 text-xs font-light text-3"
          >
            <i class="fa fa-triangle-exclamation mr-1"></i> Unsaved changes
          </span>
          <span
            v-if="row.saved"
            class="flex items-center rounded-full border border-green-300 bg-green-100/50 px-2 py-1 text-xs font-light text-green-700 dark:border-green-900 dark:bg-green-950 dark:text-green-400"
          >
            <i class="fa fa-check mr-1"></i> Saved
          </span>
          <span class="text-5 text-xs">#{{ row.hall.id }}</span>
        </div>

        <p
          v-if="row.error"
          class="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
          role="alert"
        >
          {{ row.error }}
        </p>

        <div class="grid gap-4 sm:grid-cols-2">
          <div class="flex flex-col gap-1 text-sm">
            <label :for="`lh-${row.hall.id}-name`" class="text-2">Name</label>
            <input :id="`lh-${row.hall.id}-name`" v-model="row.form.name" type="text" required class="tum-live-input" />
          </div>
          <div class="flex flex-col gap-1 text-sm">
            <span class="text-2">Stream protocol</span>
            <div class="flex h-12 items-center gap-4">
              <label class="flex items-center gap-2">
                <input v-model="row.form.streamProtocol" type="radio" :value="StreamProtocol.RTSP" />
                rtsp
              </label>
              <label class="flex items-center gap-2">
                <input v-model="row.form.streamProtocol" type="radio" :value="StreamProtocol.SRT" />
                srt
              </label>
            </div>
          </div>
        </div>

        <div class="border-t pt-3 dark:border-gray-800">
          <h2 class="text-1 font-semibold">Sources</h2>
          <p class="text-5 text-xs">
            Stream URLs the worker pulls from. Leave a source empty if the hall does not offer it.
          </p>
          <div class="mt-3 grid gap-4 sm:grid-cols-3">
            <div class="flex flex-col gap-1 text-sm">
              <label :for="`lh-${row.hall.id}-pres`" class="text-2">Presentation</label>
              <input
                :id="`lh-${row.hall.id}-pres`"
                v-model="row.form.presIp"
                type="text"
                placeholder="rtsp://0.0.0.0/pres"
                class="tum-live-input"
              />
            </div>
            <div class="flex flex-col gap-1 text-sm">
              <label :for="`lh-${row.hall.id}-cam`" class="text-2">Camera</label>
              <input
                :id="`lh-${row.hall.id}-cam`"
                v-model="row.form.camIp"
                type="text"
                placeholder="rtsp://0.0.0.0/cam"
                class="tum-live-input"
              />
            </div>
            <div class="flex flex-col gap-1 text-sm">
              <label :for="`lh-${row.hall.id}-comb`" class="text-2">Combined</label>
              <input
                :id="`lh-${row.hall.id}-comb`"
                v-model="row.form.combIp"
                type="text"
                placeholder="rtsp://0.0.0.0/comb"
                class="tum-live-input"
              />
            </div>
          </div>
        </div>

        <div class="border-t pt-3 dark:border-gray-800">
          <h2 class="text-1 font-semibold">
            Hardware <span class="text-5 font-normal">- optional</span>
          </h2>
          <p class="text-5 text-xs">
            Only for halls with the physical devices attached. VMP backed halls have neither.
          </p>
          <div class="mt-3 grid gap-4 sm:grid-cols-2">
            <div class="flex flex-col gap-1 text-sm">
              <label :for="`lh-${row.hall.id}-camera`" class="text-2">Axis camera</label>
              <input
                :id="`lh-${row.hall.id}-camera`"
                v-model="row.form.cameraIp"
                type="text"
                placeholder="10.0.0.1"
                class="tum-live-input"
              />
            </div>
            <div class="flex flex-col gap-1 text-sm">
              <label :for="`lh-${row.hall.id}-pwrctrl`" class="text-2">Anel PWR-Ctrl</label>
              <input
                :id="`lh-${row.hall.id}-pwrctrl`"
                v-model="row.form.pwrCtrlIp"
                type="text"
                placeholder="10.0.0.2"
                class="tum-live-input"
              />
            </div>
          </div>
        </div>

        <div v-if="row.hall.cameraIp" class="border-t pt-3 dark:border-gray-800">
          <div class="flex items-center gap-2">
            <h2 class="text-1 mr-auto font-semibold">Camera presets</h2>
            <button
              type="button"
              title="Reload presets"
              class="tum-live-button-secondary tum-live-button px-3 py-1 text-xs"
              :disabled="row.refreshingPresets"
              @click="reloadPresets(row)"
            >
              <i class="fa fa-sync mr-1" :class="row.refreshingPresets && 'fa-spin'"></i> Reload presets
            </button>
          </div>

          <p
            v-if="row.presetsError"
            class="mt-2 rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
            role="alert"
          >
            {{ row.presetsError }}
          </p>

          <p v-if="!row.hall.cameraPresets.length" class="text-5 mt-3 text-sm">
            No presets fetched from this camera yet.
          </p>
          <div v-else class="scrollbarThin mt-3 overflow-x-auto">
            <div class="flex flex-row gap-x-2">
              <div
                v-for="preset in row.hall.cameraPresets"
                :key="preset.presetId"
                style="min-width: 150px"
                class="group text-5 relative text-center text-sm"
              >
                <img
                  :src="presetImageUrl(preset)"
                  alt="preset preview"
                  width="150"
                  class="rounded-lg border border-slate-500 dark:border-slate-400"
                />
                <button
                  type="button"
                  title="Set default"
                  :aria-label="`Set ${preset.name} as default`"
                  class="absolute left-1 top-1 rounded bg-blue-600 p-1 text-white group-hover:opacity-100"
                  :class="!preset.isDefault && 'opacity-0'"
                  @click="makeDefault(row, preset)"
                >
                  <i class="fas fa-check"></i>
                </button>
                <button
                  type="button"
                  title="Take new snapshot"
                  :aria-label="`Take a new snapshot for ${preset.name}`"
                  class="absolute right-1 top-1 rounded bg-indigo-500 p-1 text-white opacity-0 group-hover:opacity-100"
                  :disabled="row.snapshotting === preset.presetId"
                  @click="snapshotPreset(row, preset)"
                >
                  <i class="fas fa-sync" :class="row.snapshotting === preset.presetId && 'fa-spin'"></i>
                </button>
                <span :title="preset.name" class="my-2 block truncate">{{ preset.name }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="text-5 mr-auto hover:text-red-600 dark:hover:text-red-400"
            :aria-label="`Delete ${row.hall.name}`"
            @click="remove(row)"
          >
            Delete
          </button>
          <button
            v-if="changed(row)"
            type="button"
            class="tum-live-button-secondary tum-live-button px-4 py-2 text-sm"
            @click="resetRow(row)"
          >
            Reset
          </button>
          <button
            type="submit"
            class="tum-live-button-primary px-4 py-2 text-sm"
            :disabled="!changed(row) || row.saving || !row.form.name.trim()"
          >
            {{ row.saving ? "Saving…" : "Save" }}
          </button>
        </div>
      </form>
    </section>
  </AdminLayout>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
