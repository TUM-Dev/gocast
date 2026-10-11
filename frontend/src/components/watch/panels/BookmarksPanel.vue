<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from "vue";

import { formatTime, parseTime, useBookmarks, type PanelPlayer, type WatchBookmark } from "@/lib/watch-panels";

/**
 * The viewer's own bookmarks of one recording: add one at the current time, edit,
 * delete, click to jump. Signed-out viewers get no bookmarks (the sidebar hides the
 * tab); if the session turns out to be gone anyway the panel says so.
 *
 * Emits `bookmarks` with the whole sorted list on load and after every change, so
 * the seekbar can draw markers without loading them a second time.
 */
const props = defineProps<{ streamId: number; player: PanelPlayer }>();
const emit = defineEmits<{
  bookmarks: [bookmarks: WatchBookmark[]];
}>();

const { bookmarks, loading, error, signedOut, load, add, update, remove } = useBookmarks(props.streamId);

onMounted(load);
watch(bookmarks, (list) => emit("bookmarks", list), { immediate: true });

const adding = ref(false);
const draft = ref({ description: "", time: "" });
const descriptionInput = ref<HTMLInputElement | null>(null);
const formError = ref("");

async function openAdd(): Promise<void> {
  // Prefilled with where the viewer is: the point of a bookmark is the moment they
  // just saw. They can still correct it.
  draft.value = { description: "", time: formatTime(props.player.getCurrentTime()) };
  formError.value = "";
  adding.value = true;
  await nextTick();
  descriptionInput.value?.focus();
}

async function submitAdd(): Promise<void> {
  const time = parseTime(draft.value.time);
  if (time === null) {
    formError.value = "Enter the time as m:ss or h:mm:ss.";
    return;
  }
  if (await add(draft.value.description.trim(), time)) adding.value = false;
}

const editingId = ref<number | null>(null);
const edit = ref({ description: "", time: "" });

function startEdit(b: WatchBookmark): void {
  editingId.value = b.id;
  edit.value = { description: b.description, time: formatTime(b.time) };
  formError.value = "";
}

async function submitEdit(b: WatchBookmark): Promise<void> {
  const time = parseTime(edit.value.time);
  if (time === null) {
    formError.value = "Enter the time as m:ss or h:mm:ss.";
    return;
  }
  if (await update(b.id, edit.value.description.trim(), time)) editingId.value = null;
}

const field =
  "w-full rounded border-0 bg-gray-50 px-3 py-2 text-sm text-3 focus:outline-2 focus:outline-blue-500 dark:bg-gray-600";
const iconButton =
  "h-8 w-8 rounded-full text-sm text-3 hover:bg-gray-200 focus-visible:outline-2 focus-visible:outline-blue-500 dark:hover:bg-gray-800";
</script>

<template>
  <section class="flex h-full min-h-0 flex-col" aria-label="Bookmarks">
    <header class="flex items-center justify-between p-3">
      <h3 class="text-3 text-base font-bold">Bookmarks</h3>
      <button
        v-if="!adding && !signedOut"
        type="button"
        class="text-3 rounded-full px-3 py-1 text-sm font-semibold hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-blue-500 dark:hover:bg-gray-600"
        @click="openAdd"
      >
        Add bookmark
      </button>
    </header>

    <form v-if="adding" class="grid gap-3 px-3 pb-3 text-sm" @submit.prevent="submitAdd">
      <label class="text-5 block">
        Description
        <input
          ref="descriptionInput"
          v-model="draft.description"
          type="text"
          required
          placeholder="Interesting..."
          :class="[field, 'mt-1']"
        />
      </label>
      <label class="text-5 block">
        Time (m:ss or h:mm:ss)
        <input v-model="draft.time" type="text" required inputmode="numeric" :class="[field, 'mt-1']" />
      </label>
      <div class="flex gap-2">
        <button
          type="submit"
          class="grow rounded bg-blue-500 px-3 py-2 text-sm font-semibold text-white dark:bg-indigo-600"
        >
          Add
        </button>
        <button
          type="button"
          class="text-3 rounded bg-gray-100 px-3 py-2 text-sm dark:bg-gray-700"
          @click="adding = false"
        >
          Cancel
        </button>
      </div>
    </form>

    <p v-if="formError" class="px-3 pb-2 text-sm text-red-600 dark:text-red-400" role="alert">{{ formError }}</p>
    <p v-if="signedOut" class="text-5 px-3 text-sm">Sign in to keep bookmarks.</p>
    <p v-else-if="error" class="px-3 pb-2 text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
    <p v-else-if="loading && !bookmarks.length" class="text-5 px-3 text-sm">Loading...</p>
    <p v-else-if="!bookmarks.length && !adding" class="text-5 px-3 text-sm">No bookmarks yet.</p>

    <ul class="min-h-0 grow overflow-y-auto">
      <li v-for="b in bookmarks" :key="b.id" class="group flex items-center" data-testid="bookmark">
        <form
          v-if="editingId === b.id"
          class="grid grow gap-2 px-3 py-2 text-sm"
          @submit.prevent="submitEdit(b)"
          @keydown.esc="editingId = null"
        >
          <input v-model="edit.description" type="text" required aria-label="Description" :class="field" />
          <input v-model="edit.time" type="text" required aria-label="Time" inputmode="numeric" :class="field" />
          <div class="flex gap-2">
            <button type="submit" class="grow rounded bg-blue-500 py-1 text-white dark:bg-indigo-600">Update</button>
            <button type="button" class="text-3 grow rounded bg-gray-100 py-1 dark:bg-gray-700" @click="editingId = null">
              Cancel
            </button>
          </div>
        </form>
        <template v-else>
          <button
            type="button"
            class="flex grow items-center gap-3 px-3 py-2 text-left hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-blue-500 dark:hover:bg-gray-600"
            @click="player.seek(b.time)"
          >
            <span
              class="shrink-0 rounded bg-sky-200 p-1 text-xs text-sky-800 dark:bg-indigo-800 dark:text-indigo-200"
              >{{ formatTime(b.time) }}</span
            >
            <span class="text-3 min-w-0 break-words text-sm font-semibold">{{ b.description }}</span>
          </button>
          <button type="button" :class="iconButton" :aria-label="`Edit bookmark ${b.description}`" @click="startEdit(b)">
            <i class="fa-solid fa-pen" aria-hidden="true" />
          </button>
          <button type="button" :class="iconButton" :aria-label="`Delete bookmark ${b.description}`" @click="remove(b.id)">
            <i class="fa-solid fa-trash" aria-hidden="true" />
          </button>
        </template>
      </li>
    </ul>
  </section>
</template>
