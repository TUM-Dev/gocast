import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it } from "vitest";
import { nextTick } from "vue";

import { announceDownload, dismissDownloadNotice } from "@/lib/download-notice";
import DownloadNotice from "./DownloadNotice.vue";

describe("DownloadNotice", () => {
  afterEach(dismissDownloadNotice);

  it("is only there while a download is being prepared", async () => {
    const w = mount(DownloadNotice);
    expect(w.find('[role="status"]').exists()).toBe(false);

    announceDownload();
    await nextTick();
    expect(w.find('[role="status"]').text()).toContain("Preparing your download");

    await w.find("button").trigger("click");
    expect(w.find('[role="status"]').exists()).toBe(false);
  });
});
