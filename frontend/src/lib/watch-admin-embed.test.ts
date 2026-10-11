import { describe, expect, it } from "vitest";

import { WatchViewKind } from "@/gen/server/apiv2_pb";
import { embedClip, embedKind, embedSprite, isEmbedQuery, minutesUntil, pickEmbedView } from "./watch-admin-embed";

const v = (kind: WatchViewKind) => ({ kind, playlistUrl: String(kind) }) as never;

describe("embed helpers", () => {
  it("recognises the query", () => {
    expect(isEmbedQuery({ video_only: "1" })).toBe(true);
    expect(isEmbedQuery({ video_only: "" })).toBe(true);
    expect(isEmbedQuery({ video_only: "0" })).toBe(false);
    expect(isEmbedQuery({})).toBe(false);
  });
  it("maps versions, SPLIT to combined", () => {
    expect(embedKind("PRES")).toBe(WatchViewKind.PRESENTATION);
    expect(embedKind("cam")).toBe(WatchViewKind.CAMERA);
    expect(embedKind("SPLIT")).toBe(WatchViewKind.COMBINED);
    expect(embedKind(undefined)).toBe(WatchViewKind.COMBINED);
  });
  it("falls back to combined, then the first", () => {
    const views = [v(WatchViewKind.PRESENTATION), v(WatchViewKind.COMBINED)];
    expect(pickEmbedView(views, WatchViewKind.CAMERA)?.kind).toBe(WatchViewKind.COMBINED);
    expect(pickEmbedView([views[0]!], WatchViewKind.CAMERA)?.kind).toBe(WatchViewKind.PRESENTATION);
    expect(pickEmbedView([], WatchViewKind.CAMERA)).toBeUndefined();
  });
  it("clips only a cut recording", () => {
    expect(embedClip({})).toBeUndefined();
    expect(embedClip({ startOffset: 60, endOffset: 600 })).toEqual({ start: 60, end: 600 });
    expect(embedClip({ startOffset: 60 })).toEqual({ start: 60, end: undefined });
  });
  it("builds the sprite source", () => {
    expect(embedSprite(5, { sprite: { fileId: 9, intervalSeconds: 10 } } as never)).toEqual({
      url: "/api/v2/streams/5/files/9",
      interval: 10,
    });
    expect(embedSprite(5, undefined)).toBeUndefined();
  });
  it("counts whole minutes to the start", () => {
    const now = new Date("2026-01-01T10:00:00Z");
    expect(minutesUntil(new Date("2026-01-01T10:05:30Z"), now)).toBe(5);
    expect(minutesUntil(new Date("2026-01-01T09:00:00Z"), now)).toBe(0);
    expect(minutesUntil(null, now)).toBe(0);
  });
});
