/**
 * The watch page's data and the decisions made from it.
 *
 * The page makes one getStreamWatch call and derives everything from the answer: which
 * of the stream's views to play, what to do before the stream starts, where playback
 * resumes. All of that is pure and lives here, so the view only wires it up.
 *
 * Times: the server's offsets and silences are seconds into the *recording*; the player
 * runs on the *clip's* clock when the stream is cut (see lib/player/clip.ts). The
 * helpers say which clock they take and return.
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";

import {
  StreamWatchSchema,
  WatchLayout,
  WatchViewKind,
  type StreamWatch,
  type WatchSilence,
  type WatchView,
} from "@/gen/server/apiv2_pb";
import { ApiError, apiGetMessageOptionalAuth } from "./api";
import type { Clip } from "./player/clip";
import type { SubtitleTrack } from "./player/tracks";
import { DEFAULT_PLAYBACK_RATES } from "./player/speeds";
import { streamFileUrl } from "./stream-files";
import type { CurrentUser } from "./settings";

export { WatchLayout, WatchViewKind };
export type { StreamWatch, WatchSilence, WatchView };

// ---------------------------------------------------------------- fetching

/**
 * Loads everything the page renders. Anonymous callers are served too (a public
 * course's stream), so the token is used when there is a session and skipped when
 * there is not.
 */
export async function fetchStreamWatch(slug: string, streamId: number): Promise<StreamWatch> {
  return apiGetMessageOptionalAuth(
    StreamWatchSchema,
    `/streams/${encodeURIComponent(slug)}/${streamId}/watch`,
  );
}

/** Why the page cannot show the stream. */
export type WatchFailure = "private" | "login" | "not-found" | "error";

/**
 * Maps a failed getStreamWatch to what the page says. 400 is a slug that belongs to
 * another course (getStream answers the same way), which to a visitor is a stream
 * that does not exist at this address. 401/403 differ by whether the caller could
 * sign in to fix it: an anonymous caller is offered the login, a signed-in one is told
 * the lecture is not theirs.
 */
export function classifyWatchError(err: unknown, signedIn: boolean): WatchFailure {
  if (!(err instanceof ApiError)) return "error";
  switch (err.status) {
    case 400:
    case 404:
      return "not-found";
    case 401:
      return "login";
    case 403:
      return signedIn ? "private" : "login";
    default:
      return "error";
  }
}

// ---------------------------------------------------------------- layout

/** The URL's `:version` segment for each layout. */
const LAYOUT_SEGMENTS: Record<string, WatchLayout> = {
  COMB: WatchLayout.COMBINED,
  PRES: WatchLayout.PRESENTATION,
  CAM: WatchLayout.CAMERA,
  SPLIT: WatchLayout.SPLIT,
};

export function layoutSegment(layout: WatchLayout): string | undefined {
  return Object.entries(LAYOUT_SEGMENTS).find(([, value]) => value === layout)?.[0];
}

/** The layout a `:version` names, or undefined for none or something unknown. */
export function parseLayoutSegment(version: unknown): WatchLayout | undefined {
  return typeof version === "string" ? LAYOUT_SEGMENTS[version.toUpperCase()] : undefined;
}

const KIND_OF_LAYOUT: Partial<Record<WatchLayout, WatchViewKind>> = {
  [WatchLayout.COMBINED]: WatchViewKind.COMBINED,
  [WatchLayout.PRESENTATION]: WatchViewKind.PRESENTATION,
  [WatchLayout.CAMERA]: WatchViewKind.CAMERA,
};

/** The view a single-player layout plays; SPLIT plays two and has no single view. */
export function viewForLayout(watch: StreamWatch, layout: WatchLayout): WatchView | undefined {
  const kind = KIND_OF_LAYOUT[layout];
  return kind === undefined ? undefined : watch.views.find((v) => v.kind === kind);
}

export function viewOfKind(watch: StreamWatch, kind: WatchViewKind): WatchView | undefined {
  return watch.views.find((v) => v.kind === kind);
}

function layoutOfKind(kind: WatchViewKind): WatchLayout {
  switch (kind) {
    case WatchViewKind.PRESENTATION:
      return WatchLayout.PRESENTATION;
    case WatchViewKind.CAMERA:
      return WatchLayout.CAMERA;
    default:
      return WatchLayout.COMBINED;
  }
}

/** Whether a layout can be shown with the views this stream has. */
export function layoutAvailable(watch: StreamWatch, layout: WatchLayout): boolean {
  if (layout === WatchLayout.SPLIT) return watch.splitAvailable;
  return viewForLayout(watch, layout) !== undefined;
}

/**
 * The layout to show. The URL wins where the stream can show it (anonymous viewers
 * included: the server already applied the course's source mode to `views`). Without
 * one, or when it names a view the stream lacks, the server's `defaultLayout` — which
 * already folds in the viewer's LECTURE_VIEW setting — decides; and if even that is
 * unspecified, the first view there is.
 */
export function resolveLayout(watch: StreamWatch, requested: WatchLayout | undefined): WatchLayout {
  if (requested !== undefined && layoutAvailable(watch, requested)) return requested;
  if (watch.defaultLayout !== WatchLayout.UNSPECIFIED && layoutAvailable(watch, watch.defaultLayout)) {
    return watch.defaultLayout;
  }
  const first = watch.views[0];
  return first ? layoutOfKind(first.kind) : WatchLayout.UNSPECIFIED;
}

/** The layouts the viewer can switch between, in the order the old page listed them. */
export function availableLayouts(watch: StreamWatch): WatchLayout[] {
  return [WatchLayout.COMBINED, WatchLayout.PRESENTATION, WatchLayout.CAMERA, WatchLayout.SPLIT].filter((l) =>
    layoutAvailable(watch, l),
  );
}

// ---------------------------------------------------------------- phases

/**
 * Which page to show.
 *  - `live`: being broadcast now and playable.
 *  - `vod`: a recording to play.
 *  - `waiting`: not started yet (coming up, or planned further out); shows the countdown.
 *  - `ended`: over, with nothing to play.
 */
export type WatchPhase = "live" | "vod" | "waiting" | "ended";

export function watchPhase(watch: StreamWatch): WatchPhase {
  const s = watch.stream;
  if (!s) return "ended";
  const playable = watch.views.length > 0;
  if (s.liveNow && playable) return "live";
  if (s.recording && playable) return "vod";
  if (s.ended) return "ended";
  return "waiting";
}

export interface Countdown {
  days: number;
  hours: number;
  minutes: number;
  seconds: number;
  /** The start has passed (or is this second): the stream is due. */
  due: boolean;
}

export function countdown(start: Date, now: Date): Countdown {
  const ms = start.getTime() - now.getTime();
  if (ms <= 0) return { days: 0, hours: 0, minutes: 0, seconds: 0, due: true };
  const total = Math.floor(ms / 1000);
  return {
    days: Math.floor(total / 86400),
    hours: Math.floor((total % 86400) / 3600),
    minutes: Math.floor((total % 3600) / 60),
    seconds: total % 60,
    due: false,
  };
}

const pad = (n: number) => String(n).padStart(2, "0");

/** "01:02:03", or "2 days" once it is a day or more away (a ticking clock is noise then). */
export function formatCountdown(c: Countdown): string {
  if (c.due) return "Stream is due";
  if (c.days >= 1) return c.days === 1 ? "Starts in more than a day" : `Starts in ${c.days} days`;
  return `${pad(c.hours)}:${pad(c.minutes)}:${pad(c.seconds)}`;
}

/** How long the waiting room waits before asking the server again. */
export const WAITING_POLL_MS = 30_000;
/** When the stream is due, the server flips it live a moment late: ask more often. */
export const DUE_POLL_MS = 10_000;

export function waitingPollInterval(c: Countdown): number {
  return c.due ? DUE_POLL_MS : WAITING_POLL_MS;
}

// ---------------------------------------------------------------- clip, resume, silences

/**
 * The stored cut as a player clip, or undefined when the stream is uncut. An end at or
 * before the start would play nothing, so it is treated as no end.
 */
export function offsetsToClip(watch: StreamWatch): Clip | undefined {
  const start = watch.startOffset ?? 0;
  const end = watch.endOffset;
  const hasEnd = end !== undefined && end > start;
  if (start <= 0 && !hasEnd) return undefined;
  return hasEnd ? { start, end } : { start };
}

/** Seconds on the clip's clock for a time on the recording's. */
export function toClipTime(recordingTime: number, clip: Clip | undefined): number {
  return Math.max(0, recordingTime - (clip?.start ?? 0));
}

/**
 * Progress at or above this counts as finished. The old page resumed at progress *
 * duration regardless, which put a viewer who had watched to the end on the last
 * frame; opening a finished lecture at its start is what they want.
 */
const FINISHED = 0.98;

export interface StartInput {
  /** The page's `?t=`, in clip seconds. */
  t?: number;
  /** The viewer's AUTO_SKIP setting. */
  autoSkip: boolean;
}

/**
 * Where playback starts, in clip seconds. An explicit `?t=` wins over everything. Else
 * the viewer resumes at their progress, or — when they skip silences automatically —
 * at the end of the opening silence if that is further on (the server sends the
 * latter as a fraction of the recording, see StreamWatch.first_silence_end_progress).
 * Needs the recording's length to turn a fraction into seconds; without one the
 * viewer starts at the beginning.
 */
export function resolveStartTime(watch: StreamWatch, input: StartInput): number | undefined {
  if (input.t !== undefined) return input.t;
  if (watch.stream?.recording !== true) return undefined;
  const duration = watch.stream.duration;
  if (!duration) return undefined;

  let fraction = watch.progress && watch.progress.progress < FINISHED ? watch.progress.progress : 0;
  if (input.autoSkip && watch.firstSilenceEndProgress !== undefined) {
    fraction = Math.max(fraction, watch.firstSilenceEndProgress);
  }
  if (fraction <= 0) return undefined;
  return toClipTime(fraction * duration, offsetsToClip(watch));
}

/** The silence a recording time is inside, if any. Silences are recording seconds. */
export function silenceAt(silences: readonly WatchSilence[], recordingTime: number): WatchSilence | undefined {
  return silences.find((s) => recordingTime >= s.start && recordingTime < s.end);
}

// ---------------------------------------------------------------- player options

/** Where a subtitle language's WebVTT is served. Cookie-authenticated, as <track> needs. */
export function subtitleUrl(slug: string, streamId: number, lang: string): string {
  return `/api/v2/streams/${encodeURIComponent(slug)}/${streamId}/subtitles/${encodeURIComponent(lang)}`;
}

/** "de" -> "Deutsch"; the code itself where the browser knows no name for it. */
export function languageLabel(lang: string): string {
  try {
    return new Intl.DisplayNames([lang], { type: "language" }).of(lang) ?? lang;
  } catch {
    return lang;
  }
}

export function subtitleTracks(watch: StreamWatch): SubtitleTrack[] {
  const course = watch.course;
  const stream = watch.stream;
  if (!course || !stream) return [];
  return watch.subtitleLanguages.map((lang) => ({
    src: subtitleUrl(course.slug, stream.id, lang),
    lang,
    label: languageLabel(lang),
  }));
}

/** The seek-preview sheet of a view, for the player's `sprites` prop. */
export function spriteSource(streamId: number, view: WatchView): { url: string; interval: number } | undefined {
  return view.sprite
    ? { url: streamFileUrl(streamId, view.sprite.fileId), interval: view.sprite.intervalSeconds }
    : undefined;
}

/** The speeds the viewer enabled, plus the ones they added; the defaults without a user. */
export function playbackRatesFor(user: CurrentUser | null): number[] {
  if (!user) return [...DEFAULT_PLAYBACK_RATES];
  const rates = [
    ...user.settings.playbackSpeeds.filter((s) => s.enabled).map((s) => s.speed),
    ...user.settings.customSpeeds,
  ];
  return rates.length > 0 ? rates : [...DEFAULT_PLAYBACK_RATES];
}

/** The skip buttons' jump, the viewer's SEEKING_TIME; 15 s as in the old player without one. */
export function skipSecondsFor(user: CurrentUser | null): number {
  const s = user?.settings.seekingTime;
  return s && s > 0 ? s : 15;
}

// ---------------------------------------------------------------- sidebar

export type SidebarTab = "chat" | "bookmarks" | "playlist" | "transcript" | "sections" | "search";

/**
 * Which sidebar tabs this stream and viewer have, in tab order.
 *  - chat: when the chat is on.
 *  - bookmarks, playlist: signed-in viewers (both are per user).
 *  - sections: a recording (live streams have none yet).
 *  - transcript, search: a recording with subtitles to read and search.
 */
export function sidebarTabs(watch: StreamWatch): SidebarTab[] {
  const tabs: SidebarTab[] = [];
  const recording = watch.stream?.recording === true;
  if (watch.chat?.enabled) tabs.push("chat");
  if (watch.viewer?.signedIn) tabs.push("bookmarks", "playlist");
  if (recording) tabs.push("sections");
  if (recording && watch.subtitleLanguages.length > 0) tabs.push("transcript", "search");
  return tabs;
}

export const SIDEBAR_TAB_LABELS: Record<SidebarTab, string> = {
  chat: "Chat",
  bookmarks: "Bookmarks",
  playlist: "Playlist",
  transcript: "Transcript",
  sections: "Sections",
  search: "Search",
};

// ---------------------------------------------------------------- text

/** The tab title: `Course: Lecture | TUM-Live`, like the old page's. */
export function watchDocumentTitle(watch: StreamWatch | undefined, brand = "TUM-Live"): string {
  if (!watch?.stream) return brand;
  const course = watch.course?.name;
  const name = watch.stream.displayName || watch.stream.name;
  return course ? `${brand} | ${course}: ${name}` : `${brand} | ${name}`;
}

/** The course page the header links to, `/course/{year}/{term}/{slug}`. */
export function courseHref(watch: StreamWatch): string | undefined {
  const c = watch.course;
  return c ? `/course/${c.year}/${c.term}/${c.slug}` : undefined;
}

/** "Mon, Jan 2, 15:04" for a recording's date line. */
export function formatStreamTime(start: Date): string {
  return start.toLocaleString(undefined, {
    weekday: "short",
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function streamStart(watch: StreamWatch): Date | undefined {
  return watch.stream?.start ? timestampDate(watch.stream.start) : undefined;
}

/**
 * The description is long on some lectures; show this many characters of text before
 * "Show more". Counted on the plain text, since the HTML is markup.
 */
export const DESCRIPTION_PREVIEW_CHARS = 280;

export function descriptionIsLong(html: string): boolean {
  const text = html.replace(/<[^>]*>/g, "");
  return text.length > DESCRIPTION_PREVIEW_CHARS || (html.match(/<(p|br|li)\b/gi)?.length ?? 0) > 4;
}
