/**
 * What the watch page's lecturer controls and the live management page share: the
 * camera presets, ending a stream, the links between the lecturer's pages, and the
 * arithmetic of the live display.
 */

import { EmptySchema } from "@bufbuild/protobuf/wkt";

import {
  EndStreamRequestSchema,
  ListAllowedStreamReactionsResponseSchema,
  StreamWatchSchema,
  SwitchCameraPresetRequestSchema,
  type StreamReactionStatsEvent,
  type StreamWatch,
} from "@/gen/server/apiv2_pb";
import { apiGetMessage, apiGetMessageOptionalAuth, apiPostMessage } from "./api";

/**
 * Everything the watch page renders. Anonymous callers are served on a public course,
 * so this does not insist on a session; a signed-in one gets the admin slice too.
 */
export function fetchStreamWatch(slug: string, streamId: number): Promise<StreamWatch> {
  return apiGetMessageOptionalAuth(
    StreamWatchSchema,
    `/streams/${encodeURIComponent(slug)}/${streamId}/watch`,
  );
}

/** The emoji a viewer may react with, in display order. */
export async function fetchAllowedReactions(streamId: number): Promise<string[]> {
  const res = await apiGetMessage(ListAllowedStreamReactionsResponseSchema, `/streams/${streamId}/reactions/allowed`);
  return res.reactions;
}

/** Moves the stream's camera to a preset; resolves once the camera has had time to arrive. */
export async function switchCameraPreset(courseId: number, streamId: number, presetId: number): Promise<void> {
  await apiPostMessage(
    SwitchCameraPresetRequestSchema,
    EmptySchema,
    `/courses/${courseId}/streams/${streamId}/presets/${presetId}/switch`,
    { $typeName: "protobuf.SwitchCameraPresetRequest", courseId, streamId, presetId },
  );
}

/** Ends a live stream early, keeping the recording unless `discard`. */
export async function endStream(courseId: number, streamId: number, discard: boolean): Promise<void> {
  await apiPostMessage(
    EndStreamRequestSchema,
    EmptySchema,
    `/courses/${courseId}/streams/${streamId}/end`,
    { $typeName: "protobuf.EndStreamRequest", courseId, streamId, discard },
  );
}

export interface AdminLinks {
  /** The lecture's row in the course's lectures tab. */
  edit: string;
  stats: string;
  live: string;
  /** The lecture's watch page. */
  watch: string;
}

export function adminLinks(courseId: number, streamId: number, slug: string): AdminLinks {
  const course = `/admin/courses/${courseId}`;
  return {
    edit: `${course}/lectures`,
    stats: `${course}/lectures/${streamId}/stats`,
    live: `${course}/lectures/${streamId}/live`,
    watch: `/w/${encodeURIComponent(slug)}/${streamId}`,
  };
}

export interface ReactionPercentage {
  reaction: string;
  /** "12.5%" */
  label: string;
  /** 0 to 100, for a bar. */
  percent: number;
}

/**
 * The reaction display's figures. `allowed` fixes which emoji show, and in which order,
 * so the row does not jump around as the shares change; an emoji nobody used in the
 * window reads 0%. Without `allowed` it shows what the event names.
 */
export function reactionPercentages(
  stats: Pick<StreamReactionStatsEvent, "shares"> | undefined,
  allowed?: readonly string[],
): ReactionPercentage[] {
  const share = new Map<string, number>();
  for (const s of stats?.shares ?? []) share.set(s.reaction, s.share);
  const reactions = allowed ?? [...share.keys()];
  return reactions.map((reaction) => {
    const percent = Math.min(100, Math.max(0, (share.get(reaction) ?? 0) * 100));
    return { reaction, percent, label: `${Number.isInteger(percent) ? percent : percent.toFixed(1)}%` };
  });
}

/** "1:05:09" / "12:03" until `end`, or "ended" once it has passed. */
export function timeLeft(end: Date | null, now: Date): string {
  if (!end) return "";
  const seconds = Math.floor((end.getTime() - now.getTime()) / 1000);
  if (seconds <= 0) return "ended";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
}

/** "14:03:09", in the viewer's time zone. */
export function clock(now: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`;
}
