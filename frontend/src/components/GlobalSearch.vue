<script setup lang="ts">
import { ref } from "vue";

/**
 * The header search field.
 *
 * Submitting navigates to /search, which is still server-rendered — so this is a full
 * navigation, not a route change. The live typeahead from search-global.gohtml is not
 * ported yet: it needs the Meilisearch endpoints, which exist only on the v1 API, and
 * it is course-context aware in a way that belongs with the course page. It arrives
 * with the start page.
 */
const query = ref("");

function submit(): void {
  const q = query.value.trim();
  if (q === "") {
    return;
  }
  window.location.assign(`/search?q=${encodeURIComponent(q)}`);
}
</script>

<template>
  <div
    id="search"
    class="h-10 w-60 rounded-full bg-gray-100 transition-all ease-in-out dark:bg-gray-800 sm:w-10 sm:bg-transparent sm:hover:w-96 sm:hover:bg-gray-100 sm:focus-within:w-96 sm:focus-within:bg-gray-100 sm:dark:bg-transparent sm:dark:hover:bg-gray-800 sm:dark:focus-within:bg-gray-800"
  >
    <label class="flex h-full items-center py-1 text-sm">
      <i class="fa-solid fa-search text-7 pl-3 text-xs"></i>
      <span class="sr-only">Search courses</span>
      <input
        id="search-courses"
        v-model="query"
        class="h-full w-full grow border-none bg-transparent px-3 outline-none"
        type="text"
        placeholder="Search courses"
        @keyup.enter="submit"
      />
    </label>
  </div>
</template>
