import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it } from "vitest";
import { nextTick } from "vue";
import { createMemoryHistory, createRouter, type Router } from "vue-router";

import type { OfflineLectureRecord } from "@/lib/offline/db";
import { useOfflineStore } from "@/stores/offline";
import OfflineSaveNotice from "./OfflineSaveNotice.vue";

/**
 * The save buttons sit in menus that close on the click, so this notice is the only
 * feedback a save gets: it has to show while the save runs, and say how it ended.
 */

const blank = { template: "<div />" };

async function mountAt(path: string) {
  const router: Router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/w/:slug/:id", name: "watch", component: blank },
      { path: "/offline", name: "offline", component: blank },
    ],
  });
  await router.push(path);
  return { w: mount(OfflineSaveNotice, { global: { plugins: [router] } }) };
}

const record = { streamId: 5, packageId: 9, title: "Lecture 3", state: "preparing" } as OfflineLectureRecord;

describe("OfflineSaveNotice", () => {
  beforeEach(() => setActivePinia(createPinia()));

  it("shows preparing, then the download's progress", async () => {
    const offline = useOfflineStore();
    offline.lectures = [record];
    const { w } = await mountAt("/w/eidi/5");
    expect(w.find('[data-testid="offline-save-notice"]').exists()).toBe(false);

    offline.saving = { 5: true };
    await nextTick();
    expect(w.text()).toContain("Preparing “Lecture 3” for offline viewing");

    offline.progress = { 5: 0.42 };
    await nextTick();
    expect(w.text()).toContain("Saving “Lecture 3” offline");
    expect(w.text()).toContain("42 %");
  });

  it("says how the save ended", async () => {
    const offline = useOfflineStore();
    const { w } = await mountAt("/w/eidi/5");

    offline.finished = { streamId: 5, title: "Lecture 3" };
    await nextTick();
    expect(w.find('[data-testid="offline-save-done"]').text()).toContain("Saved “Lecture 3” offline");

    offline.finished = { streamId: 5, title: "Lecture 3", error: "There isn't enough space on this device." };
    await nextTick();
    expect(w.find('[data-testid="offline-save-failed"]').text()).toContain("enough space");

    await w.find('button[aria-label="Dismiss"]').trigger("click");
    expect(w.find('[data-testid="offline-save-failed"]').exists()).toBe(false);
  });

  it("stays out of the offline page, which shows the progress itself", async () => {
    const offline = useOfflineStore();
    offline.saving = { 5: true };
    const { w } = await mountAt("/offline");
    expect(w.find('[data-testid="offline-save-notice"]').exists()).toBe(false);
  });
});
