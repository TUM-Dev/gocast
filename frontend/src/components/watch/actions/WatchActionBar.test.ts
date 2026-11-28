import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { WatchLayout, WatchViewKind } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import { dismissDownloadNotice, downloadNoticeKind, downloadNoticeVisible } from "@/lib/download-notice";
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
      controller: { getCurrentTime: () => 125.4, getDuration: () => 3600 },
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
    await byLabel(w, "Change view, current: Combined view").trigger("click");
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

  it("labels the view button with the current view", () => {
    const w = bar({ layout: WatchLayout.SPLIT });
    const btn = w.find('button[aria-label^="Change view"]');
    expect(btn.attributes("aria-label")).toBe("Change view, current: Split view");
    expect(btn.attributes("aria-haspopup")).toBe("menu");
    expect(btn.text()).toBe("Split view");
    w.unmount();
  });

  it("hides the view menu with a single view", () => {
    const w = bar({ views: [views[0]], splitAvailable: false });
    expect(w.find('button[aria-label^="Change view"]').exists()).toBe(false);
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

  it("says the download is being prepared when a recording download is clicked", async () => {
    dismissDownloadNotice();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 204 })));
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    const w = bar({
      attachments: [{ fileId: 3, filename: "slides.pdf" }],
      downloads: [{ kind: WatchViewKind.COMBINED, friendlyName: "Combined", url: "https://dl/c.mp4" }],
    });
    await byLabel(w, "Attachments and downloads").trigger("click");
    const links = w.findAll("a[role=menuitem]");
    // An attachment is a file the server already has and starts at once.
    await links[0].trigger("click");
    expect(downloadNoticeVisible.value).toBe(false);
    await links[1].trigger("click");
    expect(downloadNoticeVisible.value).toBe(true);
    dismissDownloadNotice();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    w.unmount();
  });

  it("says to retry in a minute when the edge rejects the download", async () => {
    dismissDownloadNotice();
    const fetchMock = vi.fn().mockResolvedValue(new Response("busy", { status: 503, headers: { "Retry-After": "60" } }));
    vi.stubGlobal("fetch", fetchMock);
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    const w = bar({
      downloads: [{ kind: WatchViewKind.COMBINED, friendlyName: "Combined", url: "https://dl/c.mp4?jwt=j&download=1" }],
    });
    await byLabel(w, "Attachments and downloads").trigger("click");
    await w.find("a[role=menuitem]").trigger("click");
    await vi.waitFor(() => expect(downloadNoticeKind.value).toBe("busy"));
    expect(String(fetchMock.mock.calls[0][0])).toContain("probe=1");
    // Navigating would replace the watch page with the edge's error.
    expect(click).not.toHaveBeenCalled();
    dismissDownloadNotice();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
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
    expect(body().querySelector("#watch-share-time-value")).toBeNull();
    box.click();
    await flushPromises();
    expect(body().querySelector<HTMLInputElement>("#watch-share-time-value")!.value).toBe("00:02:05");
    expect(input.value).toBe("https://tum.live/w/c/7/COMB?t=125");
    [...body().querySelectorAll("button")].find((b) => b.textContent?.includes("Copy"))!.click();
    await flushPromises();
    expect(writeText).toHaveBeenCalledWith("https://tum.live/w/c/7/COMB?t=125");
    w.unmount();
  });

  it("lets the viewer edit the start time, and the URL follows", async () => {
    const w = bar();
    await byLabel(w, "Share").trigger("click");
    const input = body().querySelector<HTMLInputElement>("#watch-share-url")!;
    body().querySelector<HTMLInputElement>("#watch-share-time")!.click();
    await flushPromises();
    const field = body().querySelector<HTMLInputElement>("#watch-share-time-value")!;

    field.value = "00:10:30";
    field.dispatchEvent(new Event("input"));
    await flushPromises();
    expect(input.value).toBe("https://tum.live/w/c/7/COMB?t=630");

    // Not a time: the URL keeps the last good one and the field is flagged.
    field.value = "10:99";
    field.dispatchEvent(new Event("input"));
    await flushPromises();
    expect(input.value).toBe("https://tum.live/w/c/7/COMB?t=630");
    expect(field.getAttribute("aria-invalid")).toBe("true");
    expect(body().textContent).toContain("Enter the time as hh:mm:ss.");

    // Past the end of the video: clamped to it, and the field shows the clamp.
    field.value = "5:00:00";
    field.dispatchEvent(new Event("input"));
    field.dispatchEvent(new Event("blur"));
    await flushPromises();
    expect(input.value).toBe("https://tum.live/w/c/7/COMB?t=3600");
    expect(field.value).toBe("01:00:00");
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
