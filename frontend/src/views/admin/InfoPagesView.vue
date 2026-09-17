<script setup lang="ts">
import { onMounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  createInfoPage,
  deleteInfoPage,
  fetchInfoPagesAdmin,
  updateInfoPage,
  type AdminInfoPage,
} from "@/lib/info-pages";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * Info page management. Privacy, imprint and about are ordinary rows here, not a
 * special case: they keep their own routes (see router/index.ts) only because their
 * URLs predate this page and must not move.
 */
const auth = useAuthStore();

const pages = ref<AdminInfoPage[]>([]);
const loading = ref(true);
const error = ref("");
const status = ref("");

/** The row currently open for editing, or null when none is. */
const editingId = ref<number | null>(null);
const editSlug = ref("");
const editName = ref("");
const editContent = ref("");
const saving = ref(false);

const newSlug = ref("");
const newName = ref("");
const newContent = ref("");
const creating = ref(false);
const showCreateForm = ref(false);

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to administer info pages.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function load(): Promise<void> {
  try {
    pages.value = await fetchInfoPagesAdmin();
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

function edit(page: AdminInfoPage): void {
  editingId.value = page.id;
  editSlug.value = page.slug;
  editName.value = page.name;
  editContent.value = page.rawContent;
  status.value = "";
}

function cancelEdit(): void {
  editingId.value = null;
}

async function saveEdit(page: AdminInfoPage): Promise<void> {
  error.value = "";
  status.value = "";
  saving.value = true;
  try {
    const updated = await updateInfoPage(
      page.id,
      editSlug.value.trim(),
      editName.value.trim(),
      editContent.value,
    );
    const index = pages.value.findIndex((p) => p.id === page.id);
    if (index !== -1) pages.value[index] = updated;
    editingId.value = null;
    status.value = `Saved "${updated.name}".`;
  } catch (err) {
    error.value = message(err);
  } finally {
    saving.value = false;
  }
}

async function remove(page: AdminInfoPage): Promise<void> {
  if (!window.confirm(`Delete "${page.name}"? Its page stops resolving immediately.`)) return;

  error.value = "";
  status.value = "";
  try {
    await deleteInfoPage(page.id);
    pages.value = pages.value.filter((p) => p.id !== page.id);
    status.value = `Deleted "${page.name}".`;
  } catch (err) {
    error.value = message(err);
  }
}

async function create(): Promise<void> {
  error.value = "";
  status.value = "";
  creating.value = true;
  try {
    const created = await createInfoPage(
      newSlug.value.trim(),
      newName.value.trim(),
      newContent.value,
    );
    pages.value.push(created);
    status.value = `Created "${created.name}". It is live at /${created.slug} now.`;
    newSlug.value = "";
    newName.value = "";
    newContent.value = "";
    showCreateForm.value = false;
  } catch (err) {
    error.value = message(err);
  } finally {
    creating.value = false;
  }
}
</script>

<template>
  <AdminLayout>
    <section class="flex w-full flex-col gap-6">
      <div class="flex items-center justify-between">
        <h1 class="text-1 text-2xl font-bold">Info Pages</h1>
        <button
          type="button"
          class="tum-live-button-primary px-4 py-2 text-sm"
          @click="showCreateForm = !showCreateForm"
        >
          {{ showCreateForm ? "Cancel" : "Add page" }}
        </button>
      </div>

      <form
        v-if="showCreateForm"
        class="flex flex-col gap-3 rounded-lg border p-4 dark:border-gray-800"
        @submit.prevent="create"
      >
        <h2 class="text-1 font-semibold">New info page</h2>
        <p class="text-5 text-sm">
          Reachable at /{slug} as soon as it is created — no deploy needed.
        </p>
        <div class="flex flex-wrap gap-3">
          <div class="flex flex-1 flex-col gap-1 text-sm">
            <label class="text-2" for="new-info-page-name">Title</label>
            <input
              id="new-info-page-name"
              v-model="newName"
              class="tum-live-input"
              autocomplete="off"
            />
          </div>
          <div class="flex flex-1 flex-col gap-1 text-sm">
            <label class="text-2" for="new-info-page-slug">Slug</label>
            <input
              id="new-info-page-slug"
              v-model="newSlug"
              class="tum-live-input"
              placeholder="e.g. terms-of-use"
              autocomplete="off"
            />
          </div>
        </div>
        <div class="flex flex-col gap-1 text-sm">
          <label class="text-2" for="new-info-page-content">Content (Markdown)</label>
          <textarea
            id="new-info-page-content"
            v-model="newContent"
            rows="8"
            class="tum-live-input font-mono text-xs"
          ></textarea>
        </div>
        <button
          type="submit"
          class="tum-live-input-submit tum-live-button-primary px-4 py-2 text-sm"
          :disabled="creating || !newName.trim() || !newSlug.trim()"
        >
          {{ creating ? "Creating…" : "Create" }}
        </button>
      </form>

      <Transition name="fade" mode="out-in">
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
          {{ error }}
        </p>
        <p v-else-if="status" class="text-5 text-sm" role="status">{{ status }}</p>
      </Transition>

      <div class="flex flex-col gap-4 rounded-lg border p-4 dark:border-gray-800">
        <p v-if="loading" class="text-5 text-sm">Loading info pages…</p>
        <p v-else-if="!pages.length" class="text-5 text-sm">No info pages yet.</p>

        <ul v-else class="flex flex-col gap-3">
          <li
            v-for="page in pages"
            :key="page.id"
            class="rounded-lg border p-4 dark:border-gray-800"
          >
            <div v-if="editingId !== page.id" class="flex items-center justify-between gap-4">
              <div>
                <p class="text-1 font-semibold">{{ page.name }}</p>
                <p class="text-5 text-sm">/{{ page.slug }}</p>
              </div>
              <div class="flex items-center gap-4">
                <button
                  type="button"
                  class="text-5 hover:text-1"
                  :title="`Edit ${page.name}`"
                  :aria-label="`Edit ${page.name}`"
                  @click="edit(page)"
                >
                  <i class="fas fa-pen"></i>
                </button>
                <button
                  type="button"
                  class="text-5 hover:text-1"
                  :title="`Delete ${page.name}`"
                  :aria-label="`Delete ${page.name}`"
                  @click="remove(page)"
                >
                  <i class="fas fa-trash"></i>
                </button>
              </div>
            </div>

            <form v-else class="flex flex-col gap-3" @submit.prevent="saveEdit(page)">
              <div class="flex flex-wrap gap-3">
                <div class="flex flex-1 flex-col gap-1 text-sm">
                  <label class="text-2" :for="`edit-name-${page.id}`">Title</label>
                  <input :id="`edit-name-${page.id}`" v-model="editName" class="tum-live-input" />
                </div>
                <div class="flex flex-1 flex-col gap-1 text-sm">
                  <label class="text-2" :for="`edit-slug-${page.id}`">Slug</label>
                  <input :id="`edit-slug-${page.id}`" v-model="editSlug" class="tum-live-input" />
                </div>
              </div>
              <div class="flex flex-col gap-1 text-sm">
                <label class="text-2" :for="`edit-content-${page.id}`">Content (Markdown)</label>
                <textarea
                  :id="`edit-content-${page.id}`"
                  v-model="editContent"
                  rows="8"
                  class="tum-live-input font-mono text-xs"
                ></textarea>
              </div>
              <div class="flex gap-3">
                <button
                  type="submit"
                  class="tum-live-input-submit tum-live-button-primary px-4 py-2 text-sm"
                  :disabled="saving || !editName.trim() || !editSlug.trim()"
                >
                  {{ saving ? "Saving…" : "Save" }}
                </button>
                <button
                  type="button"
                  class="text-5 px-4 py-2 text-sm"
                  :disabled="saving"
                  @click="cancelEdit"
                >
                  Cancel
                </button>
              </div>
            </form>
          </li>
        </ul>
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
</style>
