<script setup lang="ts">
import { computed, onMounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { fetchConfig } from "@/lib/config";
import { ApiError } from "@/lib/api";
import {
  TOKEN_SCOPES,
  createToken,
  deleteToken,
  fetchTokens,
  tokenOwner,
  type AdminToken,
  type TokenScope,
} from "@/lib/tokens";
import { Role } from "@/lib/users";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Token management: issuing and revoking the API tokens self-streaming and the admin
 * API authenticate with.
 *
 * A created token's secret is returned exactly once, by createToken, and never
 * again -- not by fetchTokens, not by re-opening this page. That is enforced
 * server-side (see apiv2/server/token_admin.go); this view only has to avoid
 * inventing a way around it, e.g. by caching a secret across a reload.
 */
const auth = useAuthStore();

const tokens = ref<AdminToken[]>([]);
const rtmpProxyUrl = ref("");
const wikiUrl = ref("");

const loading = ref(true);
const error = ref("");

const newScope = ref<TokenScope>("lecturer");
const newExpires = ref("");
const creating = ref(false);

/** Set once, by createToken; cleared only by navigating away or creating another. */
const generatedSecret = ref<string | null>(null);
const generatedScope = ref<TokenScope | null>(null);

const copied = ref<string | null>(null);
let copyTimeout: ReturnType<typeof setTimeout> | undefined;

const activeTab = ref<"OBS" | "Zoom" | "Teams">("OBS");

/** Ids with a delete in flight, so a row can disable itself. */
const pendingIds = ref<Set<number>>(new Set());
function isPending(id: number): boolean {
  return pendingIds.value.has(id);
}

/** An account holding users.manage may still not be an administrator; admin-scoped
 * tokens are for administrators specifically, same restriction the server enforces. */
const canCreateAdminScope = computed(() => auth.user?.role === Role.admin);

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to manage tokens.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function load(): Promise<void> {
  try {
    const list = await fetchTokens();
    tokens.value = list.tokens;
    rtmpProxyUrl.value = list.rtmpProxyUrl;
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

  const [config] = await Promise.all([fetchConfig().catch(() => null), load()]);
  if (config) wikiUrl.value = config.wikiUrl;
});

async function create(): Promise<void> {
  error.value = "";
  creating.value = true;
  try {
    const expires = newExpires.value ? new Date(newExpires.value) : null;
    const secret = await createToken(newScope.value, expires);
    generatedSecret.value = secret;
    generatedScope.value = newScope.value;
    newExpires.value = "";
    await load();
  } catch (err) {
    error.value = message(err);
  } finally {
    creating.value = false;
  }
}

async function remove(token: AdminToken): Promise<void> {
  if (!window.confirm(`Delete this token? Anything using it will stop authenticating immediately.`)) {
    return;
  }

  const next = new Set(pendingIds.value);
  next.add(token.id);
  pendingIds.value = next;

  try {
    await deleteToken(token.id);
    tokens.value = tokens.value.filter((t) => t.id !== token.id);
  } catch (err) {
    error.value = message(err);
  } finally {
    const after = new Set(pendingIds.value);
    after.delete(token.id);
    pendingIds.value = after;
  }
}

async function copyAndShow(value: string, key: string): Promise<void> {
  await navigator.clipboard.writeText(value);
  copied.value = key;
  clearTimeout(copyTimeout);
  copyTimeout = setTimeout(() => (copied.value = null), 1500);
}

function formatExpires(date: Date | null): string {
  if (!date) return "no expiration";
  return date.toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "2-digit" });
}

function formatLastUse(date: Date | null): string {
  if (!date) return "never used";
  return `${formatExpires(date)} ${date.toLocaleTimeString("en-GB", { hour12: false })}`;
}
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-6">
      <h1 class="text-1 text-2xl font-bold">Token Management</h1>

      <Transition name="fade" mode="out-in">
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
          {{ error }}
        </p>
      </Transition>

      <div class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800">
        <p v-if="loading" class="text-5 text-sm">Loading tokens…</p>

        <div v-else class="overflow-x-auto">
          <table class="w-full table-auto text-left text-sm">
            <thead class="text-2 text-xs uppercase tracking-wide">
              <tr>
                <th scope="col" class="py-3 pr-6">User</th>
                <th scope="col" class="px-6 py-3">Scope</th>
                <th scope="col" class="px-6 py-3">Last Used</th>
                <th scope="col" class="px-6 py-3">Expires</th>
                <th scope="col" class="px-6 py-3">Actions</th>
              </tr>
            </thead>
            <tbody class="text-3">
              <tr v-if="!tokens.length">
                <td colspan="5" class="text-5 py-6 text-center text-sm">No tokens have been issued yet.</td>
              </tr>
              <tr
                v-for="token in tokens"
                :key="token.id"
                class="border-t dark:border-gray-800"
              >
                <td class="py-3 pr-6">{{ tokenOwner(token) }}</td>
                <td class="px-6 py-3">{{ token.scope }}</td>
                <td class="px-6 py-3 whitespace-nowrap">{{ formatLastUse(token.lastUse) }}</td>
                <td class="px-6 py-3 whitespace-nowrap">{{ formatExpires(token.expires) }}</td>
                <td class="px-6 py-3">
                  <button
                    type="button"
                    class="tum-live-button tum-live-button-secondary disabled:cursor-not-allowed disabled:opacity-40"
                    :disabled="isPending(token.id)"
                    @click="remove(token)"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <form class="grid grid-cols-1 gap-3 border-t pt-4 sm:grid-cols-2 dark:border-gray-800" @submit.prevent="create">
          <label class="flex flex-col gap-1 text-sm">
            <span class="text-2">Expiration date (optional)</span>
            <input v-model="newExpires" type="datetime-local" class="tum-live-input" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span class="text-2">Scope</span>
            <select v-model="newScope" class="tum-live-input">
              <option v-for="scope in TOKEN_SCOPES" :key="scope.value" :value="scope.value" :disabled="scope.value === 'admin' && !canCreateAdminScope">
                {{ scope.label }}
              </option>
            </select>
          </label>
          <button
            type="submit"
            class="tum-live-button-primary col-span-full px-4 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50"
            :disabled="creating"
          >
            <i class="fas fa-plus mr-1"></i>{{ creating ? "Creating…" : "Create" }}
          </button>
        </form>
      </div>

      <div v-if="generatedSecret" class="rounded-lg border p-6 dark:border-gray-800">
        <div class="flex items-center justify-between">
          <h3 class="text-1 mb-2 text-lg font-semibold">Your Generated Token</h3>
          <span class="relative inline-block">
            <code
              class="block cursor-pointer overflow-x-auto rounded-md bg-gray-200 p-2 font-mono text-sm dark:bg-secondary"
              @click="copyAndShow(generatedSecret, 'generated-token-code')"
            >
              {{ generatedSecret }}
            </code>
            <span v-if="copied === 'generated-token-code'" role="status" class="copy-tooltip">Copied</span>
          </span>
        </div>
        <p class="text-3 mb-4">
          This is your generated token. Please
          <span class="relative inline-block">
            <span
              class="tum-live-button-primary cursor-pointer rounded px-4 py-1 font-bold text-white"
              @click="copyAndShow(generatedSecret, 'generated-token-button')"
            >
              Copy
            </span>
            <span v-if="copied === 'generated-token-button'" role="status" class="copy-tooltip">Copied</span>
          </span>
          it and store it securely. It will not be shown again.
          <template v-if="generatedScope === 'lecturer'"
            ><br />To use this token for self-streaming, follow the instructions below:</template
          >
        </p>

        <div v-if="generatedScope === 'lecturer'" class="rounded-lg bg-gray-100 p-4 dark:bg-gray-800">
          <div class="flex">
            <button
              v-for="tab in (['OBS', 'Zoom', 'Teams'] as const)"
              :key="tab"
              type="button"
              class="rounded-t-lg px-4 py-2"
              :class="activeTab === tab ? 'tum-live-button-primary font-bold' : 'bg-gray-200 dark:bg-secondary'"
              @click="activeTab = tab"
            >
              {{ tab }}
            </button>
          </div>
          <div class="border-t p-4 dark:border-gray-700">
            <ol v-if="activeTab === 'OBS'" class="list-inside list-decimal">
              <li class="mb-2">Open OBS.</li>
              <li class="mb-2">
                Go to <strong>File</strong> &gt; <strong>Settings</strong> &gt; <strong>Stream</strong>.
              </li>
              <li class="mb-2">
                Select the <strong>Custom</strong> service and enter the <strong>Server</strong> and
                <strong>Stream Key</strong> from below.
              </li>
              <li class="mb-2">Click <strong>Start Streaming</strong> to go live.</li>
            </ol>
            <ol v-else-if="activeTab === 'Zoom'" class="list-inside list-decimal">
              <li class="mb-2">Sign in to the Zoom web portal.</li>
              <li class="mb-2">Click <strong>Meetings</strong>.</li>
              <li class="mb-2">
                Click <strong>Schedule a Meeting</strong> and enter the required information to schedule a
                meeting.
              </li>
              <li class="mb-2">Click <strong>Save</strong> to display a set of tabs with advanced options.</li>
              <li class="mb-2">
                Click the <strong>Live Streaming</strong> tab, then click
                <strong>Configure Custom Streaming Service</strong>.
              </li>
              <li class="mb-2">
                Follow the instructions located in the green box, which were provided by your administrator.
                Contact your administrator if the instructions do not include sufficient information, or
                enable <strong>Configure live stream during the meeting</strong> to enter the details live.
              </li>
              <li class="mb-2">
                Click <strong>Save</strong> to save your livestreaming settings. The host will be able to
                livestream this meeting without needing to add these settings after the meeting begins.
              </li>
            </ol>
            <ol v-else class="list-inside list-decimal">
              <li class="mb-2">Open Microsoft Teams and join the meeting or webinar you wish to live stream.</li>
              <li class="mb-2">Add the <strong>Custom Streaming</strong> app to the meeting.</li>
              <li class="mb-2">Click <strong>Add</strong> and <strong>Save</strong>.</li>
              <li class="mb-2">
                In the right-hand panel that opens, paste the <strong>Stream URL</strong> and
                <strong>Stream Key</strong> from below.
              </li>
              <li class="mb-2">
                Click <strong>Start streaming</strong> in the lower right, then select
                <strong>Allow</strong> in the dialog box when it appears.
              </li>
              <li class="mb-2">
                You're now live streaming! Share your screen and/or use your cameras and microphones to run
                your event as you would any normal Microsoft Teams meeting.
              </li>
              <li class="mb-2">When you're finished with the event, you can stop streaming via Teams and YouTube.</li>
            </ol>

            <p class="my-4 border-t border-gray-200 dark:border-gray-500"></p>
            <p class="mb-2">
              <strong>Server:</strong>
              <span class="relative inline-block">
                <code
                  class="cursor-pointer rounded-md bg-gray-200 p-1 pt-2 font-mono text-sm dark:bg-secondary"
                  @click="copyAndShow(rtmpProxyUrl, 'rtmp-server')"
                  >{{ rtmpProxyUrl }}</code
                >
                <span v-if="copied === 'rtmp-server'" role="status" class="copy-tooltip">Copied</span>
              </span>
            </p>
            <p>
              <strong>Stream Key:</strong>
              <span class="relative inline-block">
                <code
                  class="cursor-pointer rounded-md bg-gray-200 p-1 pt-2 font-mono text-sm dark:bg-secondary"
                  @click="copyAndShow(generatedSecret, 'stream-key')"
                  >{{ generatedSecret }}</code
                >
                <span v-if="copied === 'stream-key'" role="status" class="copy-tooltip">Copied</span>
              </span>
            </p>

            <p class="text-5 px-4 pt-4 text-sm">
              You can start streaming from 15 minutes before the lecture starts and up to 15 minutes after
              the lecture ends - TUMLive automatically finds the lecture you want to stream.
            </p>
            <p class="text-5 px-4 text-sm">
              To test your setup, you can start streaming while not in a lecture and a private test stream
              will be created.
            </p>
          </div>
          <p v-if="wikiUrl" class="mt-4 italic">
            For more information, please refer to the
            <a
              :href="`${wikiUrl}/docs/beta/usage/self-streaming/`"
              target="_blank"
              rel="noopener noreferrer"
              class="text-blue-500 underline hover:text-blue-700 dark:text-blue-400"
              >self-streaming guide</a
            >.
          </p>
        </div>
      </div>
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
.copy-tooltip {
  position: absolute;
  top: -2rem;
  left: 50%;
  transform: translateX(-50%);
  border-radius: 0.25rem;
  background-color: #374151;
  padding: 0.25rem 0.5rem;
  font-size: 0.75rem;
  white-space: nowrap;
  color: #fff;
}
</style>
