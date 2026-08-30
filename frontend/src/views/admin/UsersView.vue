<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import { ApiError } from "@/lib/api";
import {
  ASSIGNABLE_ROLES,
  Role,
  SEARCH_MIN_LENGTH,
  createUser,
  deleteUser,
  fetchStaff,
  impersonate,
  roleLabel,
  searchUsers,
  searchable,
  updateUserRole,
  type AdminUser,
  type RoleValue,
} from "@/lib/users";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * User management. Two modes: idle it pages through staff, searching it reaches every
 * account with contact details masked. Which is showing decides what the rows mean,
 * so the page says so rather than leaving it to be inferred from the mask.
 */
const ROWS_PER_PAGE = 10;
/** Long enough that typing a name does not fire a request per keystroke. */
const SEARCH_DEBOUNCE_MS = 250;

const auth = useAuthStore();

const staff = ref<AdminUser[]>([]);
const results = ref<AdminUser[] | null>(null);
const page = ref(0);

const query = ref("");
const roleFilter = ref<RoleValue | undefined>(undefined);
const searching = ref(false);

const loading = ref(true);
const error = ref("");
const status = ref("");

const newName = ref("");
const newEmail = ref("");
const creating = ref(false);

/** Which list is on screen. Null results mean nobody has searched. */
const isSearch = computed(() => results.value !== null);
const shown = computed(() => results.value ?? staff.value);

const pageCount = computed(() => Math.max(1, Math.ceil(staff.value.length / ROWS_PER_PAGE)));
const visible = computed(() =>
  // Search results are shown whole, as on the old page: a search narrows enough.
  isSearch.value
    ? shown.value
    : staff.value.slice(page.value * ROWS_PER_PAGE, (page.value + 1) * ROWS_PER_PAGE),
);

/** The role a select was just changed to. */
function selectedRole(event: Event): RoleValue {
  return Number((event.target as HTMLSelectElement).value) as RoleValue;
}

function message(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    if (err.status === 403) return "You do not have permission to manage accounts.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

async function loadStaff(): Promise<void> {
  try {
    staff.value = await fetchStaff();
    // A delete can empty the last page, leaving the table blank.
    page.value = Math.min(page.value, pageCount.value - 1);
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
  await loadStaff();
});

let debounce: ReturnType<typeof setTimeout> | undefined;

// A role on its own is a valid search, so the two cannot be watched independently.
watch([query, roleFilter], () => {
  if (debounce) clearTimeout(debounce);
  debounce = setTimeout(runSearch, SEARCH_DEBOUNCE_MS);
});

async function runSearch(): Promise<void> {
  if (!searchable(query.value, roleFilter.value)) {
    // Nothing was asked for, so go back to the staff list, not an empty result.
    results.value = null;
    return;
  }

  searching.value = true;
  try {
    results.value = await searchUsers(query.value.trim(), roleFilter.value);
    error.value = "";
  } catch (err) {
    error.value = message(err);
  } finally {
    searching.value = false;
  }
}

/** Applies a change to whichever list the row came from, so the table does not jump. */
function replaceRow(updated: AdminUser): void {
  for (const list of [staff.value, results.value]) {
    if (!list) continue;
    const index = list.findIndex((user) => user.id === updated.id);
    if (index !== -1) list[index] = updated;
  }
}

function removeRow(id: number): void {
  staff.value = staff.value.filter((user) => user.id !== id);
  if (results.value) results.value = results.value.filter((user) => user.id !== id);
}

async function changeRole(user: AdminUser, role: RoleValue): Promise<void> {
  error.value = "";
  status.value = "";
  try {
    replaceRow(await updateUserRole(user.id, role));
    status.value = `${user.name} is now ${roleLabel(role).toLowerCase()}.`;
    // Promoting into or out of a staff role changes who belongs in the staff list.
    await loadStaff();
  } catch (err) {
    error.value = message(err);
  }
}

async function remove(user: AdminUser): Promise<void> {
  if (!window.confirm(`Delete the account of ${user.name}? This cannot be undone.`)) return;

  error.value = "";
  status.value = "";
  try {
    await deleteUser(user.id);
    removeRow(user.id);
    status.value = `Deleted the account of ${user.name}.`;
  } catch (err) {
    error.value = message(err);
  }
}

async function signInAs(user: AdminUser): Promise<void> {
  if (
    !window.confirm(
      `Continue as ${user.name}? You will be signed out of your own account ` +
        `and have to sign in again.`,
    )
  ) {
    return;
  }

  try {
    await impersonate(user.id);
    // The cookie is someone else's now, so leave rather than route.
    window.location.assign("/");
  } catch (err) {
    error.value = message(err);
  }
}

async function create(): Promise<void> {
  error.value = "";
  status.value = "";
  creating.value = true;
  try {
    const created = await createUser(newName.value.trim(), newEmail.value.trim());
    newName.value = "";
    newEmail.value = "";
    status.value = `Created ${created.name} and emailed an invitation to set a password.`;
    await loadStaff();
  } catch (err) {
    error.value = message(err);
  } finally {
    creating.value = false;
  }
}
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-4">
      <h1 class="text-1 text-2xl font-bold">User Management</h1>

      <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">
        {{ error }}
      </p>
      <p v-else-if="status" class="text-5 text-sm" role="status">{{ status }}</p>

      <div class="flex flex-wrap items-end gap-4">
        <div class="flex flex-col gap-1 text-sm">
          <label class="text-2" for="user-search">Search</label>
          <input
            id="user-search"
            v-model="query"
            class="tum-live-input"
            type="search"
            placeholder="Name, email or login"
          />
        </div>
        <div class="flex flex-col gap-1 text-sm">
          <label class="text-2" for="user-role-filter">Role</label>
          <select id="user-role-filter" v-model="roleFilter" class="tum-live-input">
            <option :value="undefined">All</option>
            <option v-for="role in ASSIGNABLE_ROLES" :key="role.value" :value="role.value">
              {{ role.label }}
            </option>
          </select>
        </div>
        <p v-if="searching" class="text-5 pb-2 text-sm" role="status">Searching…</p>
      </div>

      <!--
        Which list is on screen changes what the rows are, so it says so. The masking
        in particular would otherwise look like a bug rather than the point.
      -->
      <p class="text-5 text-sm">
        <span v-if="isSearch">
          Matches across every account. Emails and logins are masked here.
        </span>
        <span v-else>
          Administrators and lecturers. Search to reach every account, including
          students — {{ SEARCH_MIN_LENGTH }} characters, or pick a role.
        </span>
      </p>

      <p v-if="loading" class="text-5 text-sm">Loading accounts…</p>
      <p v-else-if="!visible.length" class="text-5 text-sm">
        {{ isSearch ? "No accounts match that search." : "No administrators or lecturers yet." }}
      </p>

      <div v-else class="overflow-x-auto">
        <table class="w-full table-auto text-left text-sm">
          <thead class="text-2 text-xs uppercase tracking-wide">
            <tr>
              <th scope="col" class="py-3 pr-6">Name</th>
              <th scope="col" class="px-6 py-3">Email</th>
              <th scope="col" class="px-6 py-3">Role</th>
              <th scope="col" class="px-6 py-3">Actions</th>
            </tr>
          </thead>
          <tbody class="text-3">
            <tr v-for="user in visible" :key="user.id" class="border-t dark:border-gray-800">
              <td class="text-1 py-3 pr-6 font-semibold">{{ user.name }}</td>
              <td class="px-6 py-3">{{ user.email || user.lrzId || "—" }}</td>
              <td class="px-6 py-3">
                <select
                  class="tum-live-input text-xs"
                  :value="user.role"
                  :aria-label="`Role of ${user.name}`"
                  @change="changeRole(user, selectedRole($event))"
                >
                  <!--
                    A role outside the three the selector offers — `generic`, or one
                    this client does not know — still has to be selectable as the
                    current value, or the row would show the wrong role.
                  -->
                  <option
                    v-if="!ASSIGNABLE_ROLES.some((r) => r.value === user.role)"
                    :value="user.role"
                  >
                    {{ roleLabel(user.role) }}
                  </option>
                  <option v-for="role in ASSIGNABLE_ROLES" :key="role.value" :value="role.value">
                    {{ role.label }}
                  </option>
                </select>
              </td>
              <td class="px-6 py-3">
                <div class="flex items-center gap-4">
                  <!-- Administrators cannot be deleted; the server refuses it too. -->
                  <button
                    v-if="user.role !== Role.admin"
                    type="button"
                    class="text-5 hover:text-1"
                    :title="`Delete ${user.name}`"
                    :aria-label="`Delete ${user.name}`"
                    @click="remove(user)"
                  >
                    <i class="fas fa-trash"></i>
                  </button>
                  <button
                    type="button"
                    class="text-5 hover:text-1"
                    :title="`Continue as ${user.name}`"
                    :aria-label="`Continue as ${user.name}`"
                    @click="signInAs(user)"
                  >
                    <i class="fas fa-user"></i>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Search results are shown whole, so paging only applies to the staff list. -->
      <div v-if="!isSearch && pageCount > 1" class="text-3 flex items-center justify-center gap-2">
        <button
          type="button"
          class="h-8 w-8 disabled:text-gray-300 dark:disabled:text-gray-600"
          aria-label="Previous page"
          :disabled="page === 0"
          @click="page -= 1"
        >
          <i class="fa fa-chevron-left text-sm"></i>
        </button>
        <span class="text-sm font-semibold">{{ page + 1 }} / {{ pageCount }}</span>
        <button
          type="button"
          class="h-8 w-8 disabled:text-gray-300 dark:disabled:text-gray-600"
          aria-label="Next page"
          :disabled="page + 1 >= pageCount"
          @click="page += 1"
        >
          <i class="fa fa-chevron-right text-sm"></i>
        </button>
      </div>

      <form class="flex max-w-md flex-col gap-3" @submit.prevent="create">
        <h2 class="text-1 font-semibold">New user</h2>
        <p class="text-5 text-sm">
          Creates a lecturer account and emails an invitation to set a password.
        </p>
        <div class="flex flex-col gap-1 text-sm">
          <label class="text-2" for="new-user-name">Name</label>
          <input
            id="new-user-name"
            v-model="newName"
            class="tum-live-input"
            autocomplete="off"
          />
        </div>
        <div class="flex flex-col gap-1 text-sm">
          <label class="text-2" for="new-user-email">Email</label>
          <input
            id="new-user-email"
            v-model="newEmail"
            class="tum-live-input"
            type="email"
            autocomplete="off"
          />
        </div>
        <button
          type="submit"
          class="tum-live-input-submit tum-live-button-primary px-4 py-2 text-sm"
          :disabled="creating || !newName.trim() || !newEmail.trim()"
        >
          {{ creating ? "Creating…" : "Create" }}
        </button>
      </form>
    </section>
  </AdminLayout>
</template>
