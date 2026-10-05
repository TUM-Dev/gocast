<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";

import {
  fetchCourseParticipants,
  inviteCourseParticipants,
  parseInviteBatch,
  type CourseParticipant,
  type InvitationResult,
  type Invitee,
} from "@/lib/course-admin";
import { courseError, useCourseAdminStore } from "@/stores/course-admin";

/**
 * The participants tab: people without a TUM login, invited to the course by email.
 * A new address gets an account and an email to set its password; one that already
 * has an account is enrolled and told.
 *
 * The old form answered with a redirect whatever happened, and ran a batch in the
 * background; inviteCourseParticipants answers once everyone is handled, so this can
 * say how each invitation went.
 */
const store = useCourseAdminStore();
const courseId = computed(() => store.course?.id ?? 0);

const participants = ref<CourseParticipant[]>([]);
const loading = ref(true);
const listError = ref("");

async function load(): Promise<void> {
  loading.value = true;
  try {
    participants.value = await fetchCourseParticipants(courseId.value);
    listError.value = "";
  } catch (err) {
    listError.value = courseError(err);
  } finally {
    loading.value = false;
  }
}

watch(courseId, (id) => id && load(), { immediate: true });

const single = reactive<Invitee>({ name: "", email: "" });
const batch = ref("");
const busy = ref(false);
const error = ref("");
const results = ref<InvitationResult[]>([]);

const parsed = computed(() => parseInviteBatch(batch.value));

async function invite(invitees: Invitee[], done: () => void): Promise<void> {
  busy.value = true;
  error.value = "";
  results.value = [];
  try {
    results.value = await inviteCourseParticipants(courseId.value, invitees);
    done();
    await load();
  } catch (err) {
    // Nobody is invited when any invitee is malformed; the server names which.
    error.value = courseError(err);
  } finally {
    busy.value = false;
  }
}

function inviteOne(): Promise<void> {
  return invite([{ name: single.name.trim(), email: single.email.trim() }], () => {
    single.name = "";
    single.email = "";
  });
}

function inviteBatch(): Promise<void> {
  if (parsed.value.badLines.length) {
    error.value = `Line ${parsed.value.badLines.join(", ")} is not "name,email". Nobody was invited.`;
    return Promise.resolve();
  }
  return invite(parsed.value.invitees, () => {
    batch.value = "";
  });
}

function outcome(result: InvitationResult): string {
  if (result.error) return result.error;
  return result.accountCreated ? "Invited; an account was created" : "Enrolled their existing account";
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <section class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800" aria-labelledby="cp-invite">
      <header>
        <h2 id="cp-invite" class="text-1 text-lg font-semibold">Invite External Participants</h2>
        <p class="text-5 text-sm">
          For people without a TUM login. They are notified about your invitation by email.
        </p>
      </header>

      <p
        v-if="error"
        class="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
        role="alert"
      >
        {{ error }}
      </p>

      <div v-if="results.length" role="status" class="text-sm">
        <p class="text-2 mb-1 font-semibold">Invitations</p>
        <ul class="flex flex-col gap-1">
          <li v-for="result in results" :key="result.email" class="flex flex-wrap gap-x-2">
            <span class="text-1 break-all">{{ result.email }}</span>
            <span :class="result.error ? 'text-red-700 dark:text-red-400' : 'text-green-700 dark:text-green-400'">{{
              outcome(result)
            }}</span>
          </li>
        </ul>
      </div>

      <form class="grid gap-3 text-sm sm:grid-cols-[1fr_1fr_auto] sm:items-end" @submit.prevent="inviteOne">
        <div class="flex flex-col gap-1">
          <label for="cp-name" class="text-2">Name</label>
          <input
            id="cp-name"
            v-model="single.name"
            type="text"
            autocomplete="off"
            placeholder="Tim"
            required
            class="tum-live-input"
          />
        </div>
        <div class="flex flex-col gap-1">
          <label for="cp-email" class="text-2">Email</label>
          <!-- type="text": the server is the judge of an address, and says which one it refused. -->
          <input
            id="cp-email"
            v-model="single.email"
            type="text"
            inputmode="email"
            autocomplete="off"
            placeholder="tim@lmu.de"
            required
            class="tum-live-input"
          />
        </div>
        <button type="submit" class="tum-live-button-primary px-4 py-2 text-sm sm:h-12" :disabled="busy">Invite</button>
      </form>

      <form class="flex flex-col gap-2 border-t pt-3 text-sm dark:border-gray-800" @submit.prevent="inviteBatch">
        <label for="cp-batch" class="text-2">Invite several at once, one <code>name,email</code> per line</label>
        <textarea
          id="cp-batch"
          v-model="batch"
          rows="4"
          class="tum-live-input font-mono"
          :placeholder="'Tim,tim69@hotmail.com\nAnja,anja@lmu.de'"
        ></textarea>
        <div class="flex items-center justify-end gap-3">
          <p v-if="parsed.invitees.length" class="text-5 text-xs">{{ parsed.invitees.length }} to invite</p>
          <button
            type="submit"
            class="tum-live-button-primary px-4 py-2 text-sm"
            :disabled="busy || !parsed.invitees.length"
          >
            Invite all
          </button>
        </div>
      </form>
    </section>

    <section class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800" aria-labelledby="cp-list">
      <h2 id="cp-list" class="text-1 text-lg font-semibold">Invited Participants</h2>

      <p v-if="listError" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ listError }}</p>
      <p v-else-if="loading && !participants.length" class="text-5 text-sm">Loading participants…</p>
      <p v-else-if="!participants.length" class="text-5 text-sm italic">Nobody has been invited yet.</p>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm" aria-label="Invited participants">
          <thead>
            <tr class="text-4 text-left">
              <th class="py-2 pr-2 font-semibold">Name</th>
              <th class="py-2 pr-2 font-semibold">Email</th>
              <th class="py-2 font-semibold">Account set up</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in participants" :key="p.id" class="border-t dark:border-gray-800">
              <td class="text-1 py-2 pr-2">{{ p.name }}</td>
              <td class="text-3 py-2 pr-2 break-all">{{ p.email }}</td>
              <td class="text-3 py-2">{{ p.accountSetUp ? "Yes" : "No" }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
