import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api";
import CameraPresets from "./CameraPresets.vue";
import EndStreamDialog from "./EndStreamDialog.vue";
import WatchAdminStrip from "./WatchAdminStrip.vue";

const api = vi.hoisted(() => ({ switchCameraPreset: vi.fn(), endStream: vi.fn() }));
vi.mock("@/lib/watch-admin", async (orig) => ({ ...(await orig<typeof import("@/lib/watch-admin")>()), ...api }));

const presets = [
  { presetId: 1, name: "Board", imageUrl: "/public/1.jpg", isDefault: true },
  { presetId: 2, name: "Lectern", imageUrl: "/public/2.jpg", isDefault: false },
] as never;
const stubs = { RouterLink: { props: ["to"], template: '<a :href="to"><slot/></a>' } };

beforeEach(() => {
  api.switchCameraPreset.mockReset().mockResolvedValue(undefined);
  api.endStream.mockReset().mockResolvedValue(undefined);
});

describe("CameraPresets", () => {
  it("moves the camera and marks the preset", async () => {
    const w = mount(CameraPresets, { props: { courseId: 3, streamId: 7, presets } });
    await w.get("[data-testid=preset-2]").trigger("click");
    await flushPromises();
    expect(api.switchCameraPreset).toHaveBeenCalledWith(3, 7, 2);
    expect(w.get("[data-testid=preset-2]").attributes("aria-pressed")).toBe("true");
  });

  it("disables the others while moving and reports a failure", async () => {
    let finish!: () => void;
    api.switchCameraPreset.mockReturnValueOnce(new Promise<void>((r) => (finish = r)));
    const w = mount(CameraPresets, { props: { courseId: 3, streamId: 7, presets } });
    await w.get("[data-testid=preset-1]").trigger("click");
    expect(w.get("[data-testid=preset-2]").attributes("disabled")).toBeDefined();
    api.switchCameraPreset.mockRejectedValueOnce(new ApiError(409, "not live"));
    finish();
    await flushPromises();
    await w.get("[data-testid=preset-2]").trigger("click");
    await flushPromises();
    expect(w.get("[role=alert]").text()).toBe("not live");
  });

  it("renders nothing without presets", () => {
    expect(mount(CameraPresets, { props: { courseId: 3, streamId: 7, presets: [] } }).html()).toBe("<!--v-if-->");
  });
});

describe("EndStreamDialog", () => {
  it.each([
    ["Yes", false],
    ["No", true],
  ])("%s ends the stream (discard=%s)", async (label, discard) => {
    const w = mount(EndStreamDialog, { props: { courseId: 3, streamId: 7 }, global: { stubs: { Teleport: true } } });
    await w.findAll("button").find((b) => b.text() === label)!.trigger("click");
    await flushPromises();
    expect(api.endStream).toHaveBeenCalledWith(3, 7, discard);
    expect(w.emitted("ended")![0]).toEqual([discard]);
    expect(w.emitted("close")).toBeTruthy();
  });

  it("cancels without ending, and stays open on failure", async () => {
    const w = mount(EndStreamDialog, { props: { courseId: 3, streamId: 7 }, global: { stubs: { Teleport: true } } });
    await w.findAll("button").find((b) => b.text() === "Cancel")!.trigger("click");
    expect(api.endStream).not.toHaveBeenCalled();
    expect(w.emitted("close")).toBeTruthy();

    const w2 = mount(EndStreamDialog, { props: { courseId: 3, streamId: 7 }, global: { stubs: { Teleport: true } } });
    api.endStream.mockRejectedValueOnce(new ApiError(500, "boom"));
    await w2.findAll("button")[0]!.trigger("click");
    await flushPromises();
    expect(w2.get("[role=alert]").text()).toBe("boom");
    expect(w2.emitted("close")).toBeFalsy();
  });
});

describe("WatchAdminStrip", () => {
  const props = (over = {}) => ({
    admin: { cameraPresets: [], canEndStream: true, ...over } as never,
    courseId: 3,
    streamId: 7,
    slug: "eidi",
    liveNow: true,
  });

  it("links to the lecturer's pages", () => {
    const w = mount(WatchAdminStrip, { props: props(), global: { stubs } });
    const hrefs = w.findAll("a").map((a) => a.attributes("href"));
    expect(hrefs).toEqual([
      "/admin/courses/3/lectures",
      "/admin/courses/3/lectures/7/stats",
      "/admin/courses/3/lectures/7/live",
    ]);
  });

  it("offers management and ending only while it makes sense", () => {
    const w = mount(WatchAdminStrip, { props: { ...props({ canEndStream: false }), liveNow: false }, global: { stubs } });
    expect(w.findAll("a")).toHaveLength(2);
    expect(w.text()).not.toContain("End stream");
  });

  it("asks before ending, and tells the page", async () => {
    const w = mount(WatchAdminStrip, { props: props(), global: { stubs } });
    await w.findAll("button").find((b) => b.text().includes("End stream"))!.trigger("click");
    expect(w.findComponent(EndStreamDialog).exists()).toBe(true);
    await w.findComponent(EndStreamDialog).vm.$emit("ended", false);
    expect(w.emitted("ended")![0]).toEqual([false]);
  });
});
