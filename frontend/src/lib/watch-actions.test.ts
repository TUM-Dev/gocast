import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { StreamIssueCategory, WatchLayout, WatchViewKind } from "@/gen/server/apiv2_pb";
import { clearToken } from "./api";
import {
  addReaction,
  attachmentLink,
  copyablePlaylistUrl,
  fetchAllowedReactions,
  fetchSeekHeatmap,
  fetchSections,
  formatClock,
  parseShareTime,
  heatmapPath,
  layoutOptions,
  reportIssue,
  reportSeek,
  seekbarFraction,
  shareUrl,
  viewForLayout,
} from "./watch-actions";

let fetchMock: ReturnType<typeof vi.fn>;

/** Answers the token request, then `reply(url, init)` for everything else. */
function respond(reply: (url: string, init?: RequestInit) => Response): void {
  fetchMock.mockImplementation((url: string, init?: RequestInit) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(new Response(JSON.stringify({ access_token: "t", expires_in: 900 })));
    }
    return Promise.resolve(reply(String(url), init));
  });
}
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
const calls = () => fetchMock.mock.calls.filter(([u]) => !String(u).endsWith("/auth/token"));

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe("layouts", () => {
  const views = [{ kind: WatchViewKind.COMBINED }, { kind: WatchViewKind.CAMERA }];

  it("offers what the views can show, split only when available", () => {
    expect(layoutOptions(views, false).map((o) => o.segment)).toEqual(["COMB", "CAM"]);
    expect(layoutOptions(views, true).map((o) => o.segment)).toEqual(["COMB", "CAM", "SPLIT"]);
  });

  it("finds the view a layout plays", () => {
    expect(viewForLayout(views, WatchLayout.CAMERA)).toBe(views[1]);
    expect(viewForLayout(views, WatchLayout.SPLIT)).toBeUndefined();
    expect(viewForLayout(views, WatchLayout.PRESENTATION)).toBeUndefined();
  });
});

describe("links", () => {
  it("strips the edge's quality placeholder", () => {
    expect(copyablePlaylistUrl("https://e/x{{quality}}.m3u8?s=1")).toBe("https://e/x.m3u8?s=1");
  });

  it("shares the page without its query, with ?t= on request", () => {
    expect(shareUrl("https://tum.live/w/c/1/COMB?rate=2#x")).toBe("https://tum.live/w/c/1/COMB");
    expect(shareUrl("https://tum.live/w/c/1/COMB?rate=2", 61.9)).toBe("https://tum.live/w/c/1/COMB?t=61");
    expect(shareUrl("https://tum.live/w/c/1", 0)).toBe("https://tum.live/w/c/1");
  });

  it("formats a clock", () => {
    expect(formatClock(3725)).toBe("01:02:05");
  });

  it("links an upload through the files endpoint and an external page as is", () => {
    expect(attachmentLink(7, { fileId: 3, filename: "a.pdf" })).toEqual({
      label: "a.pdf",
      href: "/api/v2/streams/7/files/3?download=1",
      external: false,
    });
    expect(attachmentLink(7, { fileId: 0, filename: "Site", externalUrl: "https://x.y" })).toMatchObject({
      href: "https://x.y",
      external: true,
    });
  });
});

describe("reactions", () => {
  it("lists the allowed emoji", async () => {
    respond(() => json({ reactions: ["👍", "🎉"] }));
    expect(await fetchAllowedReactions(5)).toEqual(["👍", "🎉"]);
    expect(calls()[0][0]).toBe("/api/v2/streams/5/reactions/allowed");
  });

  it("posts a reaction and reports the 429 cooldown instead of throwing", async () => {
    respond(() => json({}));
    expect(await addReaction(5, "👍")).toBe("ok");
    expect(JSON.parse(calls()[0][1].body)).toEqual({ streamId: 5, reaction: "👍" });

    respond(() => json({ message: "slow down" }, 429));
    expect(await addReaction(5, "👍")).toBe("cooldown");

    respond(() => json({ message: "boom" }, 500));
    await expect(addReaction(5, "👍")).rejects.toThrow("boom");
  });
});

describe("reportIssue", () => {
  it("sends the categories by name and trims the text", async () => {
    respond(() => json({}));
    const result = await reportIssue(5, {
      categories: [StreamIssueCategory.VIDEO, StreamIssueCategory.LIGHT],
      comment: "  black picture ",
      name: " Max ",
    });
    expect(result).toBe("ok");
    const [url, init] = calls()[0];
    expect(url).toBe("/api/v2/streams/5/issues");
    expect(JSON.parse(init.body)).toMatchObject({
      streamId: 5,
      categories: ["STREAM_ISSUE_CATEGORY_VIDEO", "STREAM_ISSUE_CATEGORY_LIGHT"],
      comment: "black picture",
      name: "Max",
    });
  });

  it("answers cooldown on 429 and throws on 503", async () => {
    respond(() => json({ message: "x" }, 429));
    expect(await reportIssue(5, { categories: [], comment: "c" })).toBe("cooldown");
    respond(() => json({ message: "no alert room" }, 503));
    await expect(reportIssue(5, { categories: [], comment: "c" })).rejects.toThrow("no alert room");
  });
});

describe("seekbar data", () => {
  it("reads sections as seconds, sorted", async () => {
    respond(() =>
      json({
        sections: [
          { description: "B", startHours: 1, startMinutes: 0, startSeconds: 5 },
          { description: "A", startHours: 0, startMinutes: 2, startSeconds: 0 },
        ],
      }),
    );
    expect(await fetchSections("c", 5)).toEqual([
      { title: "A", start: 120 },
      { title: "B", start: 3605 },
    ]);
    expect(calls()[0][0]).toBe("/api/v2/streams/c/5/sections");
  });

  it("expands the sparse heatmap buckets and is empty when there are none", async () => {
    respond(() => json({ bucketCount: 5, buckets: [{ index: 1, hits: 4 }, { index: 3, hits: 2 }, { index: 9, hits: 1 }] }));
    expect(await fetchSeekHeatmap(5)).toEqual([0, 4, 0, 2, 0]);
    respond(() => json({}));
    expect(await fetchSeekHeatmap(5)).toEqual([]);
  });

  it("draws the heatmap path: highest bucket at the top, closed along the bottom", () => {
    expect(heatmapPath([])).toBe("");
    expect(heatmapPath([0, 0])).toBe("");
    const path = heatmapPath([0, 10]);
    expect(path.startsWith("M 0,100")).toBe(true);
    expect(path.endsWith("Z")).toBe(true);
    expect(path).toContain("10.0"); // the peak sits at y = 10, the baseline at 100
  });

  it("maps seconds to a seekbar fraction within the clip", () => {
    expect(seekbarFraction(50, 200)).toBe(0.25);
    expect(seekbarFraction(150, 100, 100)).toBe(0.5);
    expect(seekbarFraction(50, 100, 100)).toBeNull();
    expect(seekbarFraction(10, NaN)).toBeNull();
  });

  it("posts a seek position", async () => {
    respond(() => json({}));
    await reportSeek(5, 12.5);
    expect(JSON.parse(calls()[0][1].body)).toEqual({ streamId: 5, position: 12.5 });
  });
});

describe("parseShareTime", () => {
  it("reads hh:mm:ss, mm:ss and plain seconds", () => {
    expect(parseShareTime("01:02:05")).toBe(3725);
    expect(parseShareTime("2:05")).toBe(125);
    expect(parseShareTime(" 125 ")).toBe(125);
  });
  it("rejects what is not a time", () => {
    for (const bad of ["", "abc", "1:2:3:4", "1:99", "-5", "1.5", "1::2"]) expect(parseShareTime(bad), bad).toBeNull();
  });
});
