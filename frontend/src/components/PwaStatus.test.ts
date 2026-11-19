import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it } from "vitest";
import { nextTick } from "vue";

import { dismissUpdate, online, updateAvailable, watchConnectivity } from "@/lib/pwa";
import PwaStatus from "./PwaStatus.vue";

describe("PwaStatus", () => {
  afterEach(() => {
    online.value = true;
    dismissUpdate();
  });

  it("explains empty pages while the browser is offline", async () => {
    watchConnectivity();
    const w = mount(PwaStatus);
    expect(w.find('[data-testid="offline-notice"]').exists()).toBe(false);

    window.dispatchEvent(new Event("offline"));
    await nextTick();
    expect(w.find('[data-testid="offline-notice"]').text()).toContain("offline");

    window.dispatchEvent(new Event("online"));
    await nextTick();
    expect(w.find('[data-testid="offline-notice"]').exists()).toBe(false);
  });

  it("offers a waiting update instead of reloading on its own", async () => {
    const w = mount(PwaStatus);
    updateAvailable.value = true;
    await nextTick();
    expect(w.find('[data-testid="update-notice"]').text()).toContain("new version");

    await w.find("button").trigger("click");
    expect(w.find('[data-testid="update-notice"]').exists()).toBe(false);
  });
});
