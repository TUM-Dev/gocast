<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  StreamProtocol,
  deleteLectureHall,
  fetchLectureHalls,
  updateLectureHall,
  type LectureHall,
  type LectureHallInput,
} from "@/lib/lecture-halls";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Lecture hall administration: list, inline edit and delete. Creating a new hall is
 * its own route/page (LectureHallCreateView), same as the page this replaces.
 *
 * Camera preset management is not part of this page yet -- see lib/lecture-halls.ts.
 */
const auth = useAuthStore();

const halls = ref<LectureHall[]>([]);
const loading = ref(true);
const error = ref("");
const filter = ref("");

/** Editable copy of a hall's fields, plus the row's own save state. */
interface Row {
  hall: LectureHall;
  form: LectureHallInput;
  saving: boolean;
  saved: boolean;
  error: string;
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
  return reactive({ hall, form: toForm(hall), saving: false, saved: false, error: "" });
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
