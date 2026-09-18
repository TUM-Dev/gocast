<script setup lang="ts">
import { onMounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  createIntegration,
  fetchIntegrations,
  revokeIntegrationKey,
  rotateIntegrationKey,
  type Integration,
} from "@/lib/integrations";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

const auth = useAuthStore();
const integrations = ref<Integration[]>([]);
const loading = ref(true);
const busy = ref(false);
const error = ref("");
const name = ref("");
const returnUrl = ref("");
const generatedKey = ref("");
const generatedId = ref(0);
const copied = ref("");

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to administer integrations.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

onMounted(async () => {
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
    return;
  }
  try {
    integrations.value = await fetchIntegrations();
  } catch (err) {
    error.value = message(err);
  } finally {
    loading.value = false;
  }
});

function beginRequest(): void {
  error.value = "";
  copied.value = "";
  busy.value = true;
}

async function create(): Promise<void> {
  beginRequest();
  try {
    const created = await createIntegration(name.value, returnUrl.value);
    integrations.value.push({
      id: created.id,
      name: created.name,
      returnUrl: created.returnUrl,
      hasKey: true,
    });
    generatedId.value = created.id;
    generatedKey.value = created.apiKey;
    name.value = "";
    returnUrl.value = "";
  } catch (err) {
    error.value = message(err);
  } finally {
    busy.value = false;
  }
}

async function rotate(integration: Integration): Promise<void> {
  beginRequest();
  try {
    generatedKey.value = await rotateIntegrationKey(integration.id);
    generatedId.value = integration.id;
    integration.hasKey = true;
  } catch (err) {
    error.value = message(err);
  } finally {
    busy.value = false;
  }
}

async function revoke(integration: Integration): Promise<void> {
  beginRequest();
  try {
    await revokeIntegrationKey(integration.id);
    integration.hasKey = false;
    if (generatedId.value === integration.id) generatedKey.value = "";
  } catch (err) {
    error.value = message(err);
  } finally {
    busy.value = false;
  }
}

async function copy(): Promise<void> {
  try {
    await navigator.clipboard.writeText(generatedKey.value);
    copied.value = "Copied.";
  } catch {
    copied.value = "Copy failed. Select the key and copy it manually.";
  }
}
</script>

<template>
  <AdminLayout>
    <section class="flex w-full flex-col gap-6">
      <div>
        <h1 class="text-1 text-2xl font-bold">Integrations</h1>
        <p class="text-5 mt-2 text-sm">Register external applications and manage their API keys.</p>
      </div>

      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
        {{ error }}
      </p>
      <p v-if="busy" class="text-5 text-sm" role="status">
        <i class="fas fa-spinner fa-spin mr-1" aria-hidden="true"></i>Working…
      </p>

      <section
        v-if="generatedKey"
        class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800"
        aria-labelledby="generated-key-title"
      >
        <h2 id="generated-key-title" class="text-1 font-semibold break-all">
          API key for
          {{ integrations.find((row) => row.id === generatedId)?.name }}
        </h2>
        <p class="text-5 text-sm">Copy this key now. It will not be shown again.</p>
        <label for="generated-integration-key" class="text-2 text-sm">API key</label>
        <div class="flex gap-3">
          <input
            id="generated-integration-key"
            class="tum-live-input min-w-0 flex-1 font-mono"
            readonly
            :value="generatedKey"
          />
          <button
            type="button"
            class="tum-live-button-primary px-4 py-2 text-sm"
            :disabled="busy"
            @click="copy"
          >
            Copy
          </button>
        </div>
        <p v-if="copied" class="text-5 text-sm" role="status">{{ copied }}</p>
      </section>

      <form
        class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800"
        @submit.prevent="create"
      >
        <h2 class="text-1 font-semibold">Register integration</h2>
        <div class="grid gap-3 md:grid-cols-2">
          <div class="flex flex-col gap-1 text-sm">
            <label class="text-2" for="integration-name">Name</label>
            <input
              id="integration-name"
              v-model="name"
              class="tum-live-input"
              maxlength="100"
              required
              autocomplete="off"
            />
          </div>
          <div class="flex flex-col gap-1 text-sm">
            <label class="text-2" for="integration-return-url">Return URL</label>
            <input
              id="integration-return-url"
              v-model="returnUrl"
              class="tum-live-input"
              type="url"
              maxlength="2048"
              required
              placeholder="https://example.org/integration/callback"
              autocomplete="off"
            />
          </div>
        </div>
        <button
          type="submit"
          class="tum-live-input-submit tum-live-button-primary px-4 py-2 text-sm"
          :disabled="busy || loading"
        >
          Register integration
        </button>
      </form>

      <section
        class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800"
        aria-labelledby="registered-integrations-title"
      >
        <h2 id="registered-integrations-title" class="text-1 font-semibold">
          Registered integrations
        </h2>
        <p v-if="loading" class="text-5 text-sm">Loading integrations…</p>
        <p v-else-if="!integrations.length && !error" class="text-5 text-sm">
          No integrations are registered.
        </p>
        <div v-else-if="integrations.length" class="overflow-x-auto">
          <table class="w-full table-auto text-left text-sm">
            <thead class="text-2 text-xs uppercase tracking-wide">
              <tr>
                <th scope="col" class="py-3 pr-6">Name</th>
                <th scope="col" class="px-6 py-3">Return URL</th>
                <th scope="col" class="px-6 py-3">Key</th>
                <th scope="col" class="px-6 py-3">Actions</th>
              </tr>
            </thead>
            <tbody class="text-3">
              <tr
                v-for="integration in integrations"
                :key="integration.id"
                class="border-t dark:border-gray-800"
              >
                <td class="py-3 pr-6">{{ integration.name }}</td>
                <td class="break-all px-6 py-3">{{ integration.returnUrl }}</td>
                <td class="px-6 py-3">
                  {{ integration.hasKey ? "Active" : "Revoked" }}
                </td>
                <td class="px-6 py-3">
                  <div class="flex gap-3">
                    <button
                      type="button"
                      class="tum-live-button-primary whitespace-nowrap px-4 py-2 text-sm"
                      :disabled="busy"
                      @click="rotate(integration)"
                    >
                      Regenerate key
                    </button>
                    <button
                      type="button"
                      class="bg-red-500 text-white hover:bg-red-600 whitespace-nowrap px-4 py-2 text-sm"
                      :disabled="busy || !integration.hasKey"
                      @click="revoke(integration)"
                    >
                      Revoke key
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </section>
  </AdminLayout>
</template>
