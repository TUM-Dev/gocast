import { defineStore } from "pinia";
import { computed, ref, shallowRef } from "vue";

import { classifyWatchError, fetchStreamWatch, sidebarTabs, type SidebarTab, type StreamWatch, type WatchFailure } from "@/lib/watch";

/**
 * The watch page's data: one getStreamWatch answer, replaced as a whole whenever the
 * page asks again (the stream went live, the waiting room's poll), and the sidebar
 * state. One store for the page, not per stream: the view loads the stream it shows
 * and the store drops what it held for another.
 */
export const useWatchStore = defineStore("watch", () => {
  // Replaced, never mutated: nothing below the response needs deep reactivity.
  const watch = shallowRef<StreamWatch | undefined>(undefined);
  const failure = ref<WatchFailure | undefined>(undefined);
  const loading = ref(false);
  const sidebar = ref<SidebarTab | null>(null);

  const tabs = computed<SidebarTab[]>(() => (watch.value ? sidebarTabs(watch.value) : []));

  let generation = 0;
  let loadedFor: string | undefined;

  /**
   * Loads a stream. For a different stream than the last this blanks the page and
   * opens the chat; for the same one (`quiet`) the old answer stays on screen until the
   * new one lands, and a failed refresh keeps it rather than replacing a working page
   * with an error.
   */
  async function load(slug: string, streamId: number, options: { quiet?: boolean; signedIn: () => Promise<boolean> }): Promise<void> {
    const key = `${slug}/${streamId}`;
    const quiet = options.quiet === true && loadedFor === key;
    const mine = ++generation;
    if (!quiet) {
      watch.value = undefined;
      failure.value = undefined;
      loading.value = true;
      sidebar.value = null;
    }
    try {
      const answer = await fetchStreamWatch(slug, streamId);
      if (mine !== generation) return; // a newer load owns the state
      watch.value = answer;
      failure.value = undefined;
      loadedFor = key;
      if (!quiet) sidebar.value = sidebarTabs(answer).includes("chat") ? "chat" : null;
      // A tab that vanished with the new answer (the chat was switched off) closes.
      else if (sidebar.value && !sidebarTabs(answer).includes(sidebar.value)) sidebar.value = null;
    } catch (err) {
      if (mine !== generation) return;
      if (quiet) return;
      failure.value = classifyWatchError(err, await options.signedIn());
    } finally {
      if (mine === generation) loading.value = false;
    }
  }

  function openSidebar(tab: SidebarTab): void {
    if (tabs.value.includes(tab)) sidebar.value = tab;
  }
  function toggleSidebar(tab: SidebarTab): void {
    if (sidebar.value === tab) sidebar.value = null;
    else openSidebar(tab);
  }
  function closeSidebar(): void {
    sidebar.value = null;
  }

  function reset(): void {
    generation++;
    watch.value = undefined;
    failure.value = undefined;
    loading.value = false;
    sidebar.value = null;
    loadedFor = undefined;
  }

  return { watch, failure, loading, sidebar, tabs, load, openSidebar, toggleSidebar, closeSidebar, reset };
});
