/**
 * The embedded player (`/w/{slug}/{id}[/{version}]?video_only=1`), which Artemis puts in
 * an iframe: which picture it plays, the cut it applies, and the "starts in" notice.
 */

import { WatchViewKind, type StreamWatch, type WatchView } from "@/gen/server/apiv2_pb";
import type { Clip } from "./player/clip";
import type { SpriteSource } from "./player/sprites";
import { streamFileUrl } from "./stream-files";

/** Whether a route's query asks for the embedded player. The old page tested only for the key. */
export function isEmbedQuery(query: Record<string, unknown>): boolean {
  const value = query.video_only;
  if (value === undefined) return false;
  const first = Array.isArray(value) ? value[0] : value;
  return first !== "0" && first !== "false";
}

/**
 * The picture a URL's version names. SPLIT, and anything unknown, play the combined one
 * as the old embed did (it had no split view).
 */
export function embedKind(version: string | undefined): WatchViewKind {
  switch (version?.toUpperCase()) {
    case "PRES":
      return WatchViewKind.PRESENTATION;
    case "CAM":
      return WatchViewKind.CAMERA;
    default:
      return WatchViewKind.COMBINED;
  }
}

/**
 * The view to play: the one asked for, else the combined one, else the first -- a lecture
 * hall set to "presentation only" has no other.
 */
export function pickEmbedView(views: readonly WatchView[], kind: WatchViewKind): WatchView | undefined {
  return (
    views.find((v) => v.kind === kind) ??
    views.find((v) => v.kind === WatchViewKind.COMBINED) ??
    views[0]
  );
}

/** The recording's stored cut, as a player clip; undefined for an uncut recording. */
export function embedClip(watch: Pick<StreamWatch, "startOffset" | "endOffset">): Clip | undefined {
  const start = watch.startOffset ?? 0;
  const end = watch.endOffset ?? 0;
  if (start <= 0 && end <= 0) return undefined;
  return { start, end: end > start ? end : undefined };
}

export function embedSprite(streamId: number, view: WatchView | undefined): SpriteSource | undefined {
  if (!view?.sprite) return undefined;
  return { url: streamFileUrl(streamId, view.sprite.fileId), interval: view.sprite.intervalSeconds };
}

/** Whole minutes until `start`, or 0 once it has begun. */
export function minutesUntil(start: Date | null, now: Date): number {
  if (!start) return 0;
  return Math.max(0, Math.floor((start.getTime() - now.getTime()) / 60_000));
}

/** Nothing to play yet: neither live nor recorded. */
export function waitingForStart(watch: StreamWatch): boolean {
  const s = watch.stream;
  return !!s && !s.liveNow && !s.recording;
}
