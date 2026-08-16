<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute } from "vue-router";

import { fetchLoginOptions, requestPasswordReset, type LoginOptions } from "@/lib/login";

const route = useRoute();

const options = ref<LoginOptions | null>(null);
const showInternalLogin = ref(true);
const resetPassword = ref(false);
const resetEmail = ref("");
const resetRequested = ref(false);
const resetError = ref("");

/**
 * A failed attempt redirects back here with ?error, because the credentials are posted
 * to Go rather than to the API — it has to set the session cookie and honour the
 * stored redirect, neither of which the client can do.
 */
const loginFailed = ref(route.query.error !== undefined);

onMounted(async () => {
  try {
    options.value = await fetchLoginOptions();
    // When single sign-on is available it is the primary route; the internal form
    // stays one click away, matching the server-rendered page.
    showInternalLogin.value = !options.value.useSaml;
  } catch {
    // If the options cannot be loaded, still offer the internal form rather than
    // leaving the user with no way to sign in.
    options.value = { useSaml: false, idpName: "", idpColor: "" };
    showInternalLogin.value = true;
  }
});

async function submitReset(): Promise<void> {
  resetError.value = "";
  try {
    await requestPasswordReset(resetEmail.value);
    resetRequested.value = true;
  } catch {
    resetError.value = "Could not send the reset email. Please try again.";
  }
}
</script>

<template>
  <section class="grid w-full content-start gap-y-5 p-6 md:w-3/4 lg:w-2/6">
    <header>
      <h1 class="text-3 font-bold">Login</h1>
    </header>

    <template v-if="options">
      <article v-if="options.useSaml" class="w-full text-center">
        <!--
          A full navigation, not a fetch: /saml/out starts an external redirect flow
          that ends with the identity provider posting back to /shib.
        -->
        <a
          href="/saml/out"
          class="tum-live-button block w-full text-white"
          :style="{ backgroundColor: options.idpColor }"
        >
          {{ options.idpName }}
        </a>
        <div v-if="!showInternalLogin" class="text-5 p-2 text-sm">
          or
          <button type="button" class="text-3 underline" @click="showInternalLogin = true">
            use an internal account
          </button>
        </div>
      </article>

      <!--
        A real form posting to Go. Keeping it a browser form submission means the
        session cookie and the post-login redirect are handled exactly as they are for
        the server-rendered page, and password managers behave normally.
      -->
      <form
        v-if="showInternalLogin && !resetPassword"
        id="loginForm"
        method="post"
        action="/login"
        class="grid gap-3"
      >
        <div class="text-sm">
          <label for="username" class="text-5 block">Username</label>
          <input
            id="username"
            type="text"
            name="username"
            autocomplete="username"
            required
            placeholder="hansi.admin"
            class="tum-live-input"
            :autofocus="!options.useSaml"
          />
        </div>
        <div class="text-sm">
          <label for="password" class="text-5 block">Password</label>
          <input
            id="password"
            type="password"
            name="password"
            autocomplete="current-password"
            required
            placeholder="**********"
            class="tum-live-input"
          />
        </div>
        <button type="submit" class="tum-live-input-submit tum-live-button-primary py-2 text-sm">
          Login
        </button>
        <p v-if="loginFailed" class="text-warn mt-2 text-sm">
          Couldn't log in. Please double check your credentials.
        </p>
        <button type="button" class="text-5 text-sm" @click="resetPassword = true">
          Reset Password
        </button>
      </form>

      <form v-if="resetPassword" class="grid gap-3" @submit.prevent="submitReset">
        <div v-if="!resetRequested" class="text-sm">
          <label for="reset-email" class="text-5 block">Username/Email</label>
          <input
            id="reset-email"
            v-model="resetEmail"
            type="text"
            autocomplete="username"
            required
            placeholder="Username"
            class="tum-live-input"
          />
        </div>
        <button
          v-if="!resetRequested"
          type="submit"
          class="tum-live-input-submit tum-live-button-primary py-2 text-sm"
        >
          Reset Password
        </button>
        <button
          type="button"
          class="text-5 text-sm"
          @click="
            resetPassword = false;
            resetRequested = false;
          "
        >
          Back to Login
        </button>
        <p v-if="resetError" class="text-warn mt-2 text-center text-sm">{{ resetError }}</p>
        <p v-if="resetRequested" class="text-success mt-2 text-center text-sm">
          We emailed you instructions to reset your password if the username you provided is
          associated with an account.
        </p>
      </form>
    </template>
  </section>
</template>
