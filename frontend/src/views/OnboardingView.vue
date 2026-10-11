<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";

import { fetchConfig } from "@/lib/config";
import { createFirstUser, firstUserErrorMessage, firstUserProblems } from "@/lib/onboarding";

/**
 * The page a fresh deployment opens with: nobody can sign in until an administrator
 * exists, so the start page sends its visitors here while the users table is empty.
 * Once an account exists the page only points at the login.
 */
const router = useRouter();

const state = ref<"checking" | "fresh" | "set-up" | "failed">("checking");
const form = reactive({ name: "", email: "", password: "" });
const touched = reactive({ name: false, email: false, password: false });
const saving = ref(false);
const error = ref("");

const problems = computed(() => firstUserProblems(form));
const canSubmit = computed(() => Object.keys(problems.value).length === 0);

onMounted(async () => {
  try {
    state.value = (await fetchConfig()).isFreshInstallation ? "fresh" : "set-up";
  } catch {
    state.value = "failed";
  }
});

async function submit(): Promise<void> {
  touched.name = touched.email = touched.password = true;
  if (!canSubmit.value) return;
  saving.value = true;
  error.value = "";
  try {
    await createFirstUser({ name: form.name.trim(), email: form.email.trim(), password: form.password });
    await router.push({ name: "login", query: { onboarded: "1" } });
  } catch (err) {
    error.value = firstUserErrorMessage(err);
    saving.value = false;
  }
}
</script>

<template>
  <section class="grid w-full content-start gap-y-5 p-6 md:w-3/4 lg:w-2/6">
    <header>
      <h1 class="text-3 font-bold">Create the first administrator</h1>
    </header>

    <p v-if="state === 'checking'" class="text-5 text-sm">One moment…</p>

    <p v-else-if="state === 'set-up'" class="text-3 text-sm" role="status">
      This deployment is already set up.
      <RouterLink to="/login" class="underline">Sign in</RouterLink> as an administrator to create
      further accounts.
    </p>

    <p v-else-if="state === 'failed'" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
      Something went wrong. Please reload the page.
    </p>

    <form v-else class="grid gap-y-4" novalidate @submit.prevent="submit">
      <p class="text-5 text-sm">
        Nobody can sign in yet. This account becomes the server administrator; it can create
        every other account afterwards.
      </p>
      <div class="text-sm">
        <label for="name" class="text-5 block">Name</label>
        <input
          id="name"
          v-model="form.name"
          type="text"
          autocomplete="name"
          autofocus
          placeholder="Erika Mustermann"
          class="tum-live-input mt-2 w-full"
          @blur="touched.name = true"
        />
        <p v-if="touched.name && problems.name" class="text-danger mt-1 text-xs">{{ problems.name }}</p>
      </div>
      <div class="text-sm">
        <label for="email" class="text-5 block">Email</label>
        <input
          id="email"
          v-model="form.email"
          type="email"
          autocomplete="email"
          placeholder="erika.mustermann@example.org"
          class="tum-live-input mt-2 w-full"
          @blur="touched.email = true"
        />
        <p v-if="touched.email && problems.email" class="text-danger mt-1 text-xs">{{ problems.email }}</p>
      </div>
      <div class="text-sm">
        <label for="password" class="text-5 block">Password</label>
        <input
          id="password"
          v-model="form.password"
          type="password"
          autocomplete="new-password"
          class="tum-live-input mt-2 w-full"
          @blur="touched.password = true"
        />
        <p v-if="touched.password && problems.password" class="text-danger mt-1 text-xs">{{ problems.password }}</p>
      </div>

      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>

      <button type="submit" class="tum-live-button tum-live-button-primary w-full" :disabled="saving">
        Finish setup
      </button>
    </form>
  </section>
</template>
