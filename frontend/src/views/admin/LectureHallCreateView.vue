<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import { StreamProtocol, createLectureHall, type LectureHallInput } from "@/lib/lecture-halls";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/** New lecture hall form, its own page/route just as the page this replaces had. */
const auth = useAuthStore();
const router = useRouter();

const form = reactive<LectureHallInput>({
  name: "",
  streamProtocol: StreamProtocol.RTSP,
  combIp: "",
  presIp: "",
  camIp: "",
  cameraIp: "",
  pwrCtrlIp: "",
});

const saving = ref(false);
const error = ref("");

onMounted(async () => {
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
  }
});

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to administer lecture halls.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function submit(): Promise<void> {
  error.value = "";
  saving.value = true;
  try {
    await createLectureHall({
      ...form,
      name: form.name.trim(),
      combIp: form.combIp.trim(),
      presIp: form.presIp.trim(),
      camIp: form.camIp.trim(),
      cameraIp: form.cameraIp.trim(),
      pwrCtrlIp: form.pwrCtrlIp.trim(),
    });
    await router.push("/admin/lecture-halls");
  } catch (err) {
    error.value = message(err);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <AdminLayout>
    <section class="mx-auto w-full max-w-4xl">
      <form
        class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800"
        @submit.prevent="submit"
      >
        <h1 class="text-1 text-2xl font-bold">New Lecture Hall</h1>

        <p
          v-if="error"
          class="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
          role="alert"
        >
          {{ error }}
        </p>

        <div class="grid gap-4 sm:grid-cols-2">
          <div class="flex flex-col gap-1 text-sm">
            <label for="lh-form-name" class="text-2"
              >Name <span class="text-red-600 dark:text-red-400" aria-hidden="true">*</span></label
            >
            <input
              id="lh-form-name"
              v-model="form.name"
              type="text"
              placeholder="FMI_HS1"
              autofocus
              required
              class="tum-live-input"
            />
            <p class="text-5 text-xs">As the hall is named in SMP, e.g. room_00_13_009A.</p>
          </div>
          <div class="flex flex-col gap-1 text-sm">
            <span class="text-2">Stream protocol</span>
            <div class="flex h-12 items-center gap-4">
              <label class="flex items-center gap-2">
                <input v-model="form.streamProtocol" type="radio" :value="StreamProtocol.RTSP" />
                rtsp
              </label>
              <label class="flex items-center gap-2">
                <input v-model="form.streamProtocol" type="radio" :value="StreamProtocol.SRT" />
                srt
              </label>
            </div>
          </div>
        </div>

        <div class="border-t pt-3 dark:border-gray-800">
          <h2 class="text-1 font-semibold">Sources</h2>
          <p class="text-5 text-xs">
            Stream URLs the worker pulls from. Fill in the ones this hall actually offers -- a
            hall with only a combined feed leaves presentation and camera empty.
          </p>
          <div class="mt-3 grid gap-4 sm:grid-cols-3">
            <div class="flex flex-col gap-1 text-sm">
              <label for="lh-form-pres" class="text-2">Presentation</label>
              <input
                id="lh-form-pres"
                v-model="form.presIp"
                type="text"
                placeholder="rtsp://0.0.0.0/pres"
                class="tum-live-input"
              />
            </div>
            <div class="flex flex-col gap-1 text-sm">
              <label for="lh-form-cam-ip" class="text-2">Camera</label>
              <input
                id="lh-form-cam-ip"
                v-model="form.camIp"
                type="text"
                placeholder="rtsp://0.0.0.0/cam"
                class="tum-live-input"
              />
            </div>
            <div class="flex flex-col gap-1 text-sm">
              <label for="lh-form-comb-ip" class="text-2">Combined</label>
              <input
                id="lh-form-comb-ip"
                v-model="form.combIp"
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
              <label for="lh-form-camera-ip" class="text-2">Axis camera</label>
              <input
                id="lh-form-camera-ip"
                v-model="form.cameraIp"
                type="text"
                placeholder="10.0.0.1"
                class="tum-live-input"
              />
              <p class="text-5 text-xs">
                IP of the camera itself - enables presets. Leave empty if there is none.
              </p>
            </div>
            <div class="flex flex-col gap-1 text-sm">
              <label for="lh-form-pwrctrl-ip" class="text-2">Anel PWR-Ctrl</label>
              <input
                id="lh-form-pwrctrl-ip"
                v-model="form.pwrCtrlIp"
                type="text"
                placeholder="10.0.0.2"
                class="tum-live-input"
              />
              <p class="text-5 text-xs">Drives the red live light. Leave empty if there is none.</p>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2">
          <RouterLink to="/admin/lecture-halls" class="tum-live-button-secondary tum-live-button px-4 py-2 text-sm">
            Cancel
          </RouterLink>
          <button
            type="submit"
            class="tum-live-button-primary px-4 py-2 text-sm"
            :disabled="saving || !form.name.trim()"
          >
            {{ saving ? "Creating…" : "Create" }}
          </button>
        </div>
      </form>
    </section>
  </AdminLayout>
</template>
