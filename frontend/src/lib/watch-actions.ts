/**
 * What the watch page's action bar and seekbar overlays need from the server, and the
 * small pure helpers around it. The generated types stop at this boundary.
 */

import { create } from "@bufbuild/protobuf";
import { EmptySchema } from "@bufbuild/protobuf/wkt";

import {
  AddStreamReactionRequestSchema,
  GetStreamSeekHeatmapResponseSchema,
  GetVideoSectionsResponseSchema,
  ListAllowedStreamReactionsResponseSchema,
  ReportStreamIssueRequestSchema,
  ReportStreamSeekRequestSchema,
  StreamIssueCategory,
  WatchLayout,
  WatchViewKind,
  type StreamWatch,
  type WatchAttachment,
  type WatchView,
} from "@/gen/server/apiv2_pb";
import { ApiError, apiGetMessage, apiGetMessageOptionalAuth, apiPostMessage, apiPostMessagePublic } from "./api";
import { streamFileUrl } from "./stream-files";

// ----- Layouts -----

/** The last path segment of /w/{slug}/{id}/{layout}. */
export type LayoutSegment = "COMB" | "PRES" | "CAM" | "SPLIT";

export interface LayoutOption {
  layout: WatchLayout;
  segment: LayoutSegment;
  label: string;
}

const LAYOUTS: Record<WatchLayout.COMBINED | WatchLayout.PRESENTATION | WatchLayout.CAMERA | WatchLayout.SPLIT, LayoutOption> = {
  [WatchLayout.COMBINED]: { layout: WatchLayout.COMBINED, segment: "COMB", label: "Combined view" },
  [WatchLayout.PRESENTATION]: { layout: WatchLayout.PRESENTATION, segment: "PRES", label: "Presentation only" },
  [WatchLayout.CAMERA]: { layout: WatchLayout.CAMERA, segment: "CAM", label: "Camera only" },
  [WatchLayout.SPLIT]: { layout: WatchLayout.SPLIT, segment: "SPLIT", label: "Split view" },
};

const KIND_LAYOUT: Partial<Record<WatchViewKind, WatchLayout.COMBINED | WatchLayout.PRESENTATION | WatchLayout.CAMERA>> = {
  [WatchViewKind.COMBINED]: WatchLayout.COMBINED,
  [WatchViewKind.PRESENTATION]: WatchLayout.PRESENTATION,
  [WatchViewKind.CAMERA]: WatchLayout.CAMERA,
};

/** The layouts the viewer can switch to, in the old menu's order. */
export function layoutOptions(views: readonly Pick<WatchView, "kind">[], splitAvailable: boolean): LayoutOption[] {
  const have = new Set(views.map((v) => KIND_LAYOUT[v.kind]));
  const order = [WatchLayout.COMBINED, WatchLayout.PRESENTATION, WatchLayout.CAMERA] as const;
  const options = order.filter((l) => have.has(l)).map((l) => LAYOUTS[l]);
  if (splitAvailable) options.push(LAYOUTS[WatchLayout.SPLIT]);
  return options;
}

/** The view a single-player layout plays; SPLIT plays two, so it has none. */
export function viewForLayout<V extends Pick<WatchView, "kind">>(views: readonly V[], layout: WatchLayout): V | undefined {
  return views.find((v) => KIND_LAYOUT[v.kind] === layout);
}

// ----- Copy HLS URL, share, attachments -----

/**
 * The playlist URL as shown to a person. A literal {{quality}} in it is the edge's to
 * fill in (see WatchView.playlist_url), which is no use in a player that gets it pasted.
 */
export function copyablePlaylistUrl(playlistUrl: string): string {
  return playlistUrl.replaceAll("{{quality}}", "");
}

/** The page's URL without its query, plus `?t=` when a start time is asked for. */
export function shareUrl(pageUrl: string, startSeconds?: number): string {
  const url = new URL(pageUrl);
  url.search = "";
  url.hash = "";
  if (startSeconds !== undefined && Number.isFinite(startSeconds) && startSeconds > 0) {
    url.searchParams.set("t", String(Math.floor(startSeconds)));
  }
  return url.toString();
}

export function formatClock(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(Math.floor(s / 3600))}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`;
}

export interface AttachmentLink {
  label: string;
  href: string;
  /** A link to another site: open it in a tab instead of downloading. */
  external: boolean;
}

export function attachmentLink(streamId: number, a: Pick<WatchAttachment, "fileId" | "filename" | "externalUrl">): AttachmentLink {
  if (a.externalUrl) return { label: a.filename, href: a.externalUrl, external: true };
  return { label: a.filename, href: streamFileUrl(streamId, a.fileId, { download: true }), external: false };
}

// ----- Reactions -----

export async function fetchAllowedReactions(streamId: number): Promise<string[]> {
  const res = await apiGetMessageOptionalAuth(
    ListAllowedStreamReactionsResponseSchema,
    `/streams/${streamId}/reactions/allowed`,
  );
  return res.reactions;
}

/** "cooldown" is the 429 the server answers while the caller's last reaction is recent. */
export async function addReaction(streamId: number, reaction: string): Promise<"ok" | "cooldown"> {
  try {
    await apiPostMessage(
      AddStreamReactionRequestSchema,
      EmptySchema,
      `/streams/${streamId}/reactions`,
      create(AddStreamReactionRequestSchema, { streamId, reaction }),
    );
    return "ok";
  } catch (err) {
    if (err instanceof ApiError && err.status === 429) return "cooldown";
    throw err;
  }
}

// ----- Report a problem -----

export const ISSUE_CATEGORIES: { value: StreamIssueCategory; label: string }[] = [
  { value: StreamIssueCategory.VIDEO, label: "Video" },
  { value: StreamIssueCategory.AUDIO, label: "Audio" },
  { value: StreamIssueCategory.CAMERA, label: "Camera" },
  { value: StreamIssueCategory.MICROPHONE, label: "Microphone" },
  { value: StreamIssueCategory.LIGHT, label: "Light" },
  { value: StreamIssueCategory.OTHER, label: "Other" },
];

export interface IssueReport {
  categories: StreamIssueCategory[];
  comment: string;
  name?: string;
  email?: string;
  phone?: string;
}

/** Resolves "cooldown" on a 429 so the form can say so, rather than fail like an outage. */
export async function reportIssue(streamId: number, report: IssueReport): Promise<"ok" | "cooldown"> {
  try {
    await apiPostMessage(
      ReportStreamIssueRequestSchema,
      EmptySchema,
      `/streams/${streamId}/issues`,
      create(ReportStreamIssueRequestSchema, {
        streamId,
        categories: report.categories,
        comment: report.comment.trim(),
        name: report.name?.trim() ?? "",
        email: report.email?.trim() ?? "",
        phone: report.phone?.trim() ?? "",
      }),
    );
    return "ok";
  } catch (err) {
    if (err instanceof ApiError && err.status === 429) return "cooldown";
    throw err;
  }
}

// ----- Seekbar data -----

export interface SeekbarSection {
  title: string;
  /** Seconds into the recording. */
  start: number;
}

export async function fetchSections(slug: string, streamId: number): Promise<SeekbarSection[]> {
  const res = await apiGetMessage(GetVideoSectionsResponseSchema, `/streams/${slug}/${streamId}/sections`);
  return res.sections
    .map((s) => ({ title: s.description, start: (s.startHours * 60 + s.startMinutes) * 60 + s.startSeconds }))
    .sort((a, b) => a.start - b.start);
}

export const HEATMAP_BUCKETS = 150;

/** Per bucket hits, index = bucket; empty when the server has too few seeks to show. */
export async function fetchSeekHeatmap(streamId: number): Promise<number[]> {
  const res = await apiGetMessageOptionalAuth(GetStreamSeekHeatmapResponseSchema, `/streams/${streamId}/seeks/heatmap`);
  if (res.buckets.length === 0 || res.bucketCount === 0) return [];
  const hits = new Array<number>(res.bucketCount).fill(0);
  for (const b of res.buckets) if (b.index < hits.length) hits[b.index] = b.hits;
  return hits;
}

/**
 * The heatmap's SVG path in a 1000x100 box (stretched to the seekbar): a smoothed
 * curve whose height follows the hits, closed along the bottom. Ported from the old
 * repeat-heatmap.ts, which was fixed at 100 buckets; this takes any number.
 */
export function heatmapPath(hits: readonly number[]): string {
  if (hits.length === 0) return "";
  const max = Math.max(...hits);
  if (max <= 0) return "";
  const scale = 90;
  const ys = hits.map((h) => scale - (scale / max) * h + (100 - scale));
  const step = 1000 / ys.length;
  let path = `M 0,100 L 0,${ys[0].toFixed(1)}`;
  for (let i = 1; i < ys.length; i++) {
    const x0 = (i - 1) * step + step / 2;
    const x1 = i * step + step / 2;
    const mid = (x0 + x1) / 2;
    path += ` C ${mid.toFixed(1)},${ys[i - 1].toFixed(1)} ${mid.toFixed(1)},${ys[i].toFixed(1)} ${x1.toFixed(1)},${ys[i].toFixed(1)}`;
  }
  return `${path} L 1000,${ys[ys.length - 1].toFixed(1)} L 1000,100 Z`;
}

/** Seconds into the recording to a fraction of the (clipped) player's length, or null if outside it. */
export function seekbarFraction(seconds: number, duration: number, offset = 0): number | null {
  if (!(duration > 0)) return null;
  const f = (seconds - offset) / duration;
  return f >= 0 && f <= 1 ? f : null;
}

export async function reportSeek(streamId: number, position: number): Promise<void> {
  await apiPostMessagePublic(
    ReportStreamSeekRequestSchema,
    EmptySchema,
    `/streams/${streamId}/seeks`,
    create(ReportStreamSeekRequestSchema, { streamId, position }),
  );
}

/** Typing aid for the shell. */
export type WatchActionsData = Pick<StreamWatch, "views" | "splitAvailable" | "attachments" | "downloads" | "viewer">;
