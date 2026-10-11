import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { WatchLayout, WatchViewKind } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import { onOwnReaction } from "@/lib/reaction-floats";
import WatchActionBar from "./WatchActionBar.vue";

const { fetchAllowedReactions, addReaction, reportIssue } = vi.hoisted(() => ({
  fetchAllowedReactions: vi.fn(),
  addReaction: vi.fn(),
  reportIssue: vi.fn(),
}));
vi.mock("@/lib/watch-actions", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/watch-actions")>()),
  fetchAllowedReactions,
  addReaction,
  reportIssue,
}));

const views = [
  { kind: WatchViewKind.COMBINED, playlistUrl: "https://e/comb{{quality}}.m3u8" },
  { kind: WatchViewKind.CAMERA, playlistUrl: "https://e/cam{{quality}}.m3u8" },
  { kind: WatchViewKind.PRESENTATION, playlistUrl: "https://e/pres{{quality}}.m3u8" },
];

function bar(props: Record<string, unknown> = {}) {
  return mount(WatchActionBar, {
    attachTo: document.body,
    props: {
      streamId: 7,
      views,
      splitAvailable: true,
      attachments: [],
      downloads: [],
      viewer: { signedIn: true, isCourseAdmin: false, canReportIssue: true },
      layout: WatchLayout.COMBINED,
      live: false,
      controller: { getCurrentTime: () => 125.4 },
      pageUrl: "https://tum.live/w/c/7/COMB?rate=2",
      ...props,
    } as never,
  });
}
const byLabel = (w: ReturnType<typeof bar>, label: string) => w.find(`button[aria-label="${label}"]`);
const body = () => document.body;

let writeText: ReturnType<typeof vi.fn>;
beforeEach(() => {
  writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  fetchAllowedReactions.mockReset().mockResolvedValue(["👍", "🎉"]);
  addReaction.mockReset().mockResolvedValue("ok");
  reportIssue.mockReset().mockResolvedValue("ok");
});
afterEach(() => {
  document.body.innerHTML = "";
});

describe("WatchActionBar", () => {
  it("copies the current view's playlist without the quality placeholder", async () => {
    const w = bar({ layout: WatchLayout.CAMERA });
    await byLabel(w, "Copy HLS URL").trigger("click");
    expect(writeText).toHaveBeenCalledWith("https://e/cam.m3u8");
    w.unmount();
  });

  it("has no HLS button in split view, where there are two playlists", () => {
    const w = bar({ layout: WatchLayout.SPLIT });
    expect(byLabel(w, "Copy HLS URL").exists()).toBe(false);
    w.unmount();
  });

  it("emits the chosen layout and marks the current one", async () => {
    const w = bar();
    await byLabel(w, "Change view").trigger("click");
    const items = w.findAll("[role=menuitemradio]");
    expect(items.map((i) => i.text())).toEqual([
      "Combined view",
      "Presentation only",
      "Camera only",
      "Split view",
    ]);
    expect(items[0].attributes("aria-checked")).toBe("true");
    await items[3].trigger("click");
    expect(w.emitted("select-layout")?.[0][0]).toMatchObject({ segment: "SPLIT" });
    w.unmount();
  });

  it("hides the view menu with a single view", () => {
    const w = bar({ views: [views[0]], splitAvailable: false });
    expect(byLabel(w, "Change view").exists()).toBe(false);
    w.unmount();
  });

  it("lists attachments and downloads", async () => {
    const w = bar({
      attachments: [
        { fileId: 3, filename: "slides.pdf" },
        { fileId: 0, filename: "Wiki", externalUrl: "https://wiki" },
      ],
      downloads: [{ kind: WatchViewKind.COMBINED, friendlyName: "Combined", url: "https://dl/c.mp4" }],
    });
    await byLabel(w, "Attachments and downloads").trigger("click");
    const links = w.findAll("a[role=menuitem]");
    expect(links.map((l) => l.attributes("href"))).toEqual([
      "/api/v2/streams/7/files/3?download=1",
      "https://wiki",
      "https://dl/c.mp4",
    ]);
    expect(links[1].attributes("target")).toBe("_blank");
    w.unmount();
  });

  it("hides the menu without any", () => {
    const w = bar();
    expect(byLabel(w, "Attachments and downloads").exists()).toBe(false);
    w.unmount();
  });

  it("shares the page without its query and can start at the current time", async () => {
    const w = bar();
    await byLabel(w, "Share").trigger("click");
    const input = body().querySelector<HTMLInputElement>("#watch-share-url")!;
    expect(input.value).toBe("https://tum.live/w/c/7/COMB");
    const box = body().querySelector<HTMLInputElement>("#watch-share-time")!;
    expect(body().textContent).toContain("Start at 00:02:05");
    box.click();
    await flushPromises();
    expect(input.value).toBe("https://tum.live/w/c/7/COMB?t=125");
    [...body().querySelectorAll("button")].find((b) => b.textContent?.includes("Copy"))!.click();
    await flushPromises();
    expect(writeText).toHaveBeenCalledWith("https://tum.live/w/c/7/COMB?t=125");
    w.unmount();
  });

  it("offers no start time for a live stream", async () => {
    const w = bar({ live: true });
    await byLabel(w, "Share").trigger("click");
    expect(body().querySelector("#watch-share-time")).toBeNull();
    w.unmount();
  });

  it("opens the keyboard shortcuts", async () => {
    const w = bar();
    await byLabel(w, "Keyboard shortcuts").trigger("click");
    expect(body().textContent).toContain("Keyboard Shortcuts");
    w.unmount();
  });

  it("reports a problem only when the viewer can", async () => {
    const off = bar({ viewer: { signedIn: true, isCourseAdmin: false, canReportIssue: false } });
    expect(byLabel(off, "Report a problem").exists()).toBe(false);
    off.unmount();

    const w = bar();
    await byLabel(w, "Report a problem").trigger("click");
    const dialog = body().querySelector("[role=dialog][aria-label='Report a technical problem']")!;
    expect(dialog).not.toBeNull();
    (dialog.querySelector("input[type=checkbox]") as HTMLInputElement).click();
    const comment = dialog.querySelector("textarea")!;
    comment.value = "no sound";
    comment.dispatchEvent(new Event("input"));
    dialog.querySelector("form")!.dispatchEvent(new Event("submit"));
    await flushPromises();
    expect(reportIssue).toHaveBeenCalledWith(7, expect.objectContaining({ comment: "no sound", categories: [1] }));
    expect(body().textContent).toContain("report was sent");
    w.unmount();
  });

  it("does not send an empty report and shows a failure", async () => {
    reportIssue.mockRejectedValue(new ApiError(503, "no alert room"));
    const w = bar();
    await byLabel(w, "Report a problem").trigger("click");
    const form = body().querySelector("form")!;
    form.dispatchEvent(new Event("submit"));
    await flushPromises();
    expect(reportIssue).not.toHaveBeenCalled();
    const comment = form.querySelector("textarea")!;
    comment.value = "x";
    comment.dispatchEvent(new Event("input"));
    form.dispatchEvent(new Event("submit"));
    await flushPromises();
    expect(body().querySelector("[role=alert]")?.textContent).toContain("not available");
    w.unmount();
  });

  it("shows reactions to signed-in viewers of a live stream only", () => {
    expect(byLabel(bar({ live: false }), "React to stream").exists()).toBe(false);
    expect(
      byLabel(bar({ live: true, viewer: { signedIn: false, isCourseAdmin: false, canReportIssue: false } }), "React to stream").exists(),
    ).toBe(false);
    expect(byLabel(bar({ live: true }), "React to stream").exists()).toBe(true);
  });

  it("sends a reaction and explains the cooldown", async () => {
    const w = bar({ live: true });
    await byLabel(w, "React to stream").trigger("click");
    await flushPromises();
    const emoji = () => [...body().querySelectorAll<HTMLButtonElement>("[aria-label=Reactions] button")];
    expect(emoji().map((b) => b.textContent?.trim())).toEqual(["👍", "🎉"]);

    addReaction.mockResolvedValueOnce("cooldown");
    emoji()[0].click();
    await flushPromises();
    expect(body().textContent).toContain("Please wait before reacting again.");

    const own = vi.fn();
    const off = onOwnReaction(7, own);
    emoji()[1].click();
    await flushPromises();
    off();
    expect(addReaction).toHaveBeenLastCalledWith(7, "🎉");
    expect(own).toHaveBeenCalledWith("🎉");
    expect(body().querySelector("[aria-label=Reactions]")).toBeNull();
    w.unmount();
  });

  it("closes a dialog with Escape", async () => {
    const w = bar();
    await byLabel(w, "Share").trigger("click");
    body().querySelector("[data-testid=action-dialog]")!.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    await flushPromises();
    expect(body().querySelector("[data-testid=action-dialog]")).toBeNull();
    w.unmount();
  });
});
