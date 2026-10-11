import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";

import { StreamWatchSchema } from "@/gen/server/apiv2_pb";
import { ApiError } from "./api";
import {
  WatchLayout as L,
  WatchViewKind as K,
  classifyWatchError,
  countdown,
  formatCountdown,
  offsetsToClip,
  parseLayoutSegment,
  resolveLayout,
  resolveStartTime,
  sidebarTabs,
  silenceAt,
  subtitleTracks,
  watchDocumentTitle,
  watchPhase,
  type StreamWatch,
} from "./watch";

function mk(init: Record<string, unknown> = {}): StreamWatch {
  return create(StreamWatchSchema, {
    stream: { id: 7, displayName: "Intro", recording: true, duration: 1000 },
    course: { id: 1, name: "Course", slug: "c", year: 2026, term: "W" },
    views: [
      { kind: K.COMBINED, playlistUrl: "c.m3u8" },
      { kind: K.PRESENTATION, playlistUrl: "p.m3u8" },
      { kind: K.CAMERA, playlistUrl: "v.m3u8" },
    ],
    defaultLayout: L.COMBINED,
    splitAvailable: true,
    ...init,
  } as never);
}

describe("layout", () => {
  it("parses the version segment", () => {
    expect(parseLayoutSegment("PRES")).toBe(L.PRESENTATION);
    expect(parseLayoutSegment("split")).toBe(L.SPLIT);
    expect(parseLayoutSegment("nope")).toBeUndefined();
    expect(parseLayoutSegment(undefined)).toBeUndefined();
  });

  it("honours the URL, else the server default, else the first view", () => {
    expect(resolveLayout(mk(), L.CAMERA)).toBe(L.CAMERA);
    expect(resolveLayout(mk({ defaultLayout: L.PRESENTATION }), undefined)).toBe(L.PRESENTATION);
    const pres = mk({ views: [{ kind: K.PRESENTATION, playlistUrl: "p" }], defaultLayout: L.UNSPECIFIED, splitAvailable: false });
    // CAM is not offered by this stream, SPLIT neither.
    expect(resolveLayout(pres, L.CAMERA)).toBe(L.PRESENTATION);
    expect(resolveLayout(pres, L.SPLIT)).toBe(L.PRESENTATION);
  });
});

describe("watchPhase", () => {
  it("tells the states apart", () => {
    expect(watchPhase(mk())).toBe("vod");
    expect(watchPhase(mk({ stream: { liveNow: true } }))).toBe("live");
    expect(watchPhase(mk({ stream: { isComingUp: true }, views: [] }))).toBe("waiting");
    expect(watchPhase(mk({ stream: { ended: true }, views: [] }))).toBe("ended");
  });
});

describe("countdown", () => {
  it("splits and formats", () => {
    const now = new Date("2026-01-01T00:00:00Z");
    const c = countdown(new Date(now.getTime() + 3723_000), now);
    expect(c).toMatchObject({ hours: 1, minutes: 2, seconds: 3, due: false });
    expect(formatCountdown(c)).toBe("01:02:03");
    expect(formatCountdown(countdown(now, now))).toBe("Stream is due");
    expect(formatCountdown(countdown(new Date(now.getTime() + 3 * 86400_000), now))).toBe("Starts in 3 days");
  });
});

describe("offsetsToClip", () => {
  it("is absent for an uncut stream and ignores an end before the start", () => {
    expect(offsetsToClip(mk())).toBeUndefined();
    expect(offsetsToClip(mk({ startOffset: 60, endOffset: 600 }))).toEqual({ start: 60, end: 600 });
    expect(offsetsToClip(mk({ startOffset: 60, endOffset: 10 }))).toEqual({ start: 60 });
    expect(offsetsToClip(mk({ endOffset: 500 }))).toEqual({ start: 0, end: 500 });
  });
});

describe("resolveStartTime", () => {
  it("prefers ?t=, then resumes, then skips the opening silence when asked", () => {
    const w = mk({ progress: { progress: 0.5 }, firstSilenceEndProgress: 0.1 });
    expect(resolveStartTime(w, { t: 12, autoSkip: true })).toBe(12);
    expect(resolveStartTime(w, { autoSkip: false })).toBe(500);
    const fresh = mk({ firstSilenceEndProgress: 0.1 });
    expect(resolveStartTime(fresh, { autoSkip: true })).toBe(100);
    expect(resolveStartTime(fresh, { autoSkip: false })).toBeUndefined();
  });

  it("restarts a finished recording and works on the clip's clock", () => {
    expect(resolveStartTime(mk({ progress: { progress: 1, watched: true } }), { autoSkip: false })).toBeUndefined();
    expect(resolveStartTime(mk({ progress: { progress: 0.5 }, startOffset: 100 }), { autoSkip: false })).toBe(400);
  });

  it("needs the recording's length", () => {
    const w = mk({ stream: { recording: true }, progress: { progress: 0.5 } });
    expect(resolveStartTime(w, { autoSkip: false })).toBeUndefined();
  });
});

describe("misc", () => {
  it("finds the silence under the playhead", () => {
    const s = [{ start: 0, end: 5 }, { start: 10, end: 20 }] as never[];
    expect(silenceAt(s, 3)?.end).toBe(5);
    expect(silenceAt(s, 5)).toBeUndefined();
    expect(silenceAt(s, 10)?.end).toBe(20);
  });

  it("builds subtitle tracks", () => {
    const t = subtitleTracks(mk({ subtitleLanguages: ["de"] }));
    expect(t[0]).toMatchObject({ src: "/api/v2/streams/c/7/subtitles/de", lang: "de" });
  });

  it("lists tabs by what the viewer and stream have", () => {
    expect(sidebarTabs(mk({ chat: { enabled: true }, viewer: { signedIn: true }, subtitleLanguages: ["en"] }))).toEqual([
      "chat", "bookmarks", "playlist", "sections", "transcript", "search",
    ]);
    expect(sidebarTabs(mk({ stream: { liveNow: true } }))).toEqual([]);
  });

  it("offers the search with the transcript: a recording that has subtitles, whatever the search service's state", () => {
    // Meilisearch being down is the panel's "not available", not a reason to hide the tab.
    expect(sidebarTabs(mk({ subtitleLanguages: ["en"] }))).toContain("search");
    expect(sidebarTabs(mk())).not.toContain("search");
    expect(sidebarTabs(mk({ stream: { recording: false }, subtitleLanguages: ["en"] }))).not.toContain("search");
  });

  it("titles the tab", () => {
    expect(watchDocumentTitle(mk(), "B")).toBe("B | Course: Intro");
    expect(watchDocumentTitle(undefined, "B")).toBe("B");
  });

  it("classifies failures", () => {
    expect(classifyWatchError(new ApiError(400, ""), true)).toBe("not-found");
    expect(classifyWatchError(new ApiError(404, ""), true)).toBe("not-found");
    expect(classifyWatchError(new ApiError(403, ""), true)).toBe("private");
    expect(classifyWatchError(new ApiError(403, ""), false)).toBe("login");
    expect(classifyWatchError(new ApiError(500, ""), false)).toBe("error");
  });
});
