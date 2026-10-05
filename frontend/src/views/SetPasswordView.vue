<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";

import {
  checkPasswordResetKey,
  passwordProblem,
  setPasswordByResetKey,
  setPasswordErrorMessage,
} from "@/lib/password-reset";

/**
 * The page a password reset or account invite links to. The key in the URL is the
 * credential; the server is asked whether it still opens anything before the form
 * shows, where the template redirected home without a word.
 */
const props = defineProps<{ key: string }>();
const router = useRouter();

const state = ref<"checking" | "ready" | "invalid" | "failed">("checking");
const password = ref("");
const confirmation = ref("");
const saving = ref(false);
const error = ref("");

const problem = computed(() => passwordProblem(password.value, confirmation.value));

onMounted(async () => {
  try {
    state.value = (await checkPasswordResetKey(props.key)) ? "ready" : "invalid";
  } catch {
    state.value = "failed";
  }
});

async function submit(): Promise<void> {
  if (problem.value) return;
  saving.value = true;
  error.value = "";
  try {
    await setPasswordByResetKey(props.key, password.value);
    await router.push({ name: "login", query: { passwordSet: "1" } });
  } catch (err) {
    error.value = setPasswordErrorMessage(err);
    saving.value = false;
  }
}
</script>

<template>
  <section class="grid w-full content-start gap-y-5 p-6 md:w-3/4 lg:w-2/6">
    <header>
      <h1 class="text-3 font-bold">Set your password</h1>
    </header>

    <p v-if="state === 'checking'" class="text-5 text-sm">Checking your link…</p>

    <p v-else-if="state === 'invalid'" class="text-3 text-sm" role="alert">
      This link is not valid any more: it was already used, or it was never issued. You can
      request a new one from the <RouterLink to="/login" class="underline">login page</RouterLink>.
    </p>

    <p v-else-if="state === 'failed'" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
      Something went wrong. Please reload the page.
    </p>

    <form v-else class="grid gap-y-4" @submit.prevent="submit">
      <div class="text-sm">
        <label for="password" class="text-5 block">Password</label>
        <input
          id="password"
          v-model="password"
          type="password"
          autocomplete="new-password"
          required
          class="tum-live-input mt-2 w-full"
        />
      </div>
      <div class="text-sm">
        <label for="password-confirmation" class="text-5 block">Confirm</label>
        <input
          id="password-confirmation"
          v-model="confirmation"
          type="password"
          autocomplete="new-password"
          required
          class="tum-live-input mt-2 w-full"
        />
      </div>

      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>
      <p v-else-if="problem && (password || confirmation)" class="text-5 text-sm">{{ problem }}</p>

      <button type="submit" class="tum-live-button tum-live-button-primary w-full" :disabled="saving || problem !== ''">
        Set password
      </button>
    </form>
  </section>
</template>
