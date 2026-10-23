import { describe, expect, it } from "vitest";

import { WatchViewKind } from "@/gen/server/apiv2_pb";
import type { OfflineLectureRecord } from "./db";
import { localSource } from "./source";

const NOW = Date.UTC(2026, 9, 23, 12);

const saved = {
  streamId: 7,
  kind: WatchViewKind.COMBINED,
  state: "saved",
  expiresAt: NOW + 60_000,
} as OfflineLectureRecord;

describe("localSource", () => {
  it("plays the saved view from the device", () => {
    expect(localSource(saved, WatchViewKind.COMBINED, true, NOW)).toBe("/offline-media/7/playlist.m3u8");
  });

  it("leaves other views, and lectures not saved, to the network", () => {
    expect(localSource(saved, WatchViewKind.CAMERA, true, NOW)).toBeNull();
    expect(localSource(undefined, WatchViewKind.COMBINED, true, NOW)).toBeNull();
    expect(localSource({ ...saved, state: "downloading" }, WatchViewKind.COMBINED, true, NOW)).toBeNull();
  });

  it("needs a service worker to answer, and a licence that holds", () => {
    expect(localSource(saved, WatchViewKind.COMBINED, false, NOW)).toBeNull();
    expect(localSource(saved, WatchViewKind.COMBINED, true, saved.expiresAt)).toBeNull();
  });
});
