/**
 * The data behind the watch page's sidebar panels: bookmarks, sections, playlist and
 * subtitle search. The panels (components/watch/panels) only draw and emit; every
 * request and every bit of arithmetic lives here so it can be tested without a DOM.
 */

import { fromJson, type JsonValue } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { ref, type Ref } from "vue";

import {
  AddBookmarkResponseSchema,
  GetBookmarksResponseSchema,
  GetStreamPlaylistResponseSchema,
  GetVideoSectionsResponseSchema,
  SearchStreamSubtitlesResponseSchema,
  UpdateBookmarkResponseSchema,
  type Bookmark as BookmarkMessage,
} from "@/gen/server/apiv2_pb";
import { ApiError, apiDelete, apiFetch, apiFetchOptionalAuth, apiGetMessage, apiGetMessageOptionalAuth } from "./api";
import type { PlayerController } from "./player/controller";

/**
 * What the panels need of the player: the real controller's clock, seek and events.
 * A narrow Pick so tests (and the split view) can hand over anything that keeps time.
 */
export type PanelPlayer = Pick<PlayerController, "getCurrentTime" | "seek" | "on">;

const JSON_READ_OPTIONS = { ignoreUnknownFields: true } as const;

// ---------------------------------------------------------------------------
// Time
// ---------------------------------------------------------------------------

/** 75 -> "1:15", 3725 -> "1:02:05". */
export function formatTime(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(Number.isFinite(totalSeconds) ? totalSeconds : 0));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${m}:${pad(sec)}`;
}

/** The inverse of formatTime ("75", "1:15", "1:02:05"); null for anything else. */
export function parseTime(text: string): number | null {
  const parts = text.trim().split(":");
  if (parts.length < 1 || parts.length > 3 || parts.some((p) => !/^\d{1,5}$/.test(p))) return null;
  return parts.map(Number).reduce((total, part) => total * 60 + part, 0);
}

/** Seconds into the recording as the hours/minutes/seconds the bookmark RPCs use. */
export function splitTime(totalSeconds: number): { hours: number; minutes: number; seconds: number } {
  const s = Math.max(0, Math.floor(totalSeconds));
  return { hours: Math.floor(s / 3600), minutes: Math.floor((s % 3600) / 60), seconds: s % 60 };
}

function joinTime(hours: number, minutes: number, seconds: number): number {
  return hours * 3600 + minutes * 60 + seconds;
}

// ---------------------------------------------------------------------------
// Bookmarks
// ---------------------------------------------------------------------------

export interface WatchBookmark {
  id: number;
  description: string;
  /** Seconds into the recording. */
  time: number;
}

function parseBookmark(m: BookmarkMessage): WatchBookmark {
  return { id: m.bookmarkId, description: m.description, time: joinTime(m.hours, m.minutes, m.seconds) };
}

function byTime(a: WatchBookmark, b: WatchBookmark): number {
  return a.time - b.time || a.id - b.id;
}

/**
 * The add and update RPCs declare no `body` in the proto, so the gateway reads their
 * fields from the query string and throws the JSON body away. Sending a body, as the
 * other POSTs do, would create a bookmark with an empty description at 0:00.
 */
function bookmarkQuery(fields: Record<string, string | number>): string {
  return new URLSearchParams(Object.entries(fields).map(([k, v]) => [k, String(v)])).toString();
}

export async function fetchBookmarks(streamId: number): Promise<WatchBookmark[]> {
  const res = await apiGetMessage(GetBookmarksResponseSchema, `/bookmarks?${bookmarkQuery({ stream_id: streamId })}`);
  return res.bookmarks.map(parseBookmark).sort(byTime);
}

export async function createBookmark(streamId: number, description: string, time: number): Promise<WatchBookmark> {
  const { hours, minutes, seconds } = splitTime(time);
  const json = await apiFetch<JsonValue>(
    `/bookmarks?${bookmarkQuery({ stream_id: streamId, description, hours, minutes, seconds })}`,
    { method: "POST" },
  );
  const res = fromJson(AddBookmarkResponseSchema, json, JSON_READ_OPTIONS);
  if (!res.bookmark) throw new Error("the server returned no bookmark");
  return parseBookmark(res.bookmark);
}

export async function saveBookmark(id: number, description: string, time: number): Promise<WatchBookmark> {
  const { hours, minutes, seconds } = splitTime(time);
  const json = await apiFetch<JsonValue>(
    `/bookmarks/${id}?${bookmarkQuery({ description, hours, minutes, seconds })}`,
    { method: "PUT" },
  );
  const res = fromJson(UpdateBookmarkResponseSchema, json, JSON_READ_OPTIONS);
  if (!res.bookmark) throw new Error("the server returned no bookmark");
  return parseBookmark(res.bookmark);
}

export function removeBookmark(id: number): Promise<void> {
  return apiDelete(`/bookmarks/${id}`);
}

/**
 * A stream's bookmarks, loaded once and kept current as the viewer adds, edits and
 * deletes them. The seekbar's markers read `bookmarks` too, so the sidebar and the
 * bar never disagree about what exists.
 *
 * Failures land in `error` rather than throwing, so a flaky request shows a message
 * in the panel instead of an unhandled rejection; `signedOut` separates the one
 * failure the viewer can fix.
 */
export function useBookmarks(streamId: number) {
  const bookmarks: Ref<WatchBookmark[]> = ref([]);
  const loading = ref(false);
  const error = ref("");
  const signedOut = ref(false);

  async function run<T>(fn: () => Promise<T>): Promise<T | undefined> {
    error.value = "";
    try {
      return await fn();
    } catch (err) {
      if (err instanceof ApiError && err.isUnauthenticated) signedOut.value = true;
      error.value = err instanceof Error ? err.message : "Something went wrong.";
      return undefined;
    }
  }

  async function load(): Promise<void> {
    loading.value = true;
    const list = await run(() => fetchBookmarks(streamId));
    if (list) bookmarks.value = list;
    loading.value = false;
  }

  async function add(description: string, time: number): Promise<boolean> {
    const created = await run(() => createBookmark(streamId, description, time));
    if (!created) return false;
    bookmarks.value = [...bookmarks.value, created].sort(byTime);
    return true;
  }

  async function update(id: number, description: string, time: number): Promise<boolean> {
    const saved = await run(() => saveBookmark(id, description, time));
    if (!saved) return false;
    bookmarks.value = bookmarks.value.map((b) => (b.id === id ? saved : b)).sort(byTime);
    return true;
  }

  async function remove(id: number): Promise<boolean> {
    const done = await run(async () => {
      await removeBookmark(id);
      return true;
    });
    if (!done) return false;
    bookmarks.value = bookmarks.value.filter((b) => b.id !== id);
    return true;
  }

  return { bookmarks, loading, error, signedOut, load, add, update, remove };
}

// ---------------------------------------------------------------------------
// Sections
// ---------------------------------------------------------------------------

export interface WatchSection {
  description: string;
  /** Seconds into the recording. */
  time: number;
  fileId: number;
}

export async function fetchSections(slug: string, streamId: number): Promise<WatchSection[]> {
  const res = await apiGetMessageOptionalAuth(
    GetVideoSectionsResponseSchema,
    `/streams/${encodeURIComponent(slug)}/${streamId}/sections`,
  );
  return res.sections
    .map((s) => ({
      description: s.description,
      time: joinTime(s.startHours, s.startMinutes, s.startSeconds),
      fileId: s.fileId,
    }))
    .sort((a, b) => a.time - b.time);
}

/**
 * Index of the section playing at `time` (the last one that has started), or -1
 * before the first. `sections` must be sorted by time, as fetchSections returns them.
 */
export function currentIndex(starts: { time: number }[], time: number): number {
  let found = -1;
  for (let i = 0; i < starts.length; i++) {
    if (starts[i].time <= time) found = i;
    else break;
  }
  return found;
}

// ---------------------------------------------------------------------------
// Playlist
// ---------------------------------------------------------------------------

export interface WatchPlaylistEntry {
  streamId: number;
  slug: string;
  name: string;
  /** 0 to 1. */
  progress: number;
  watched: boolean;
}

/**
 * The lectures of the course that can be watched now, oldest first as the server
 * orders them. A live lecture and one that has not started are left out, as the old
 * page did — except the one being watched, which must stay so it can be highlighted.
 */
export async function fetchPlaylist(
  slug: string,
  streamId: number,
  now: Date = new Date(),
): Promise<WatchPlaylistEntry[]> {
  const res = await apiGetMessageOptionalAuth(
    GetStreamPlaylistResponseSchema,
    `/streams/${encodeURIComponent(slug)}/${streamId}/playlist`,
  );
  return res.entries
    .filter((e) => e.streamId === streamId || (!e.liveNow && (!e.start || timestampDate(e.start) < now)))
    .map((e) => ({
      streamId: e.streamId,
      slug: e.courseSlug || slug,
      name: e.streamName,
      progress: Math.min(1, Math.max(0, e.streamProgress?.progress ?? 0)),
      watched: e.watched || (e.streamProgress?.watched ?? false),
    }));
}

/** The entries before and after the stream being watched, either may be missing. */
export function neighbours(
  entries: WatchPlaylistEntry[],
  streamId: number,
): { prev: WatchPlaylistEntry | null; next: WatchPlaylistEntry | null } {
  const i = entries.findIndex((e) => e.streamId === streamId);
  if (i < 0) return { prev: null, next: null };
  return { prev: entries[i - 1] ?? null, next: entries[i + 1] ?? null };
}

// ---------------------------------------------------------------------------
// Subtitle search
// ---------------------------------------------------------------------------

export interface SubtitleHit {
  /** Seconds into the recording. */
  time: number;
  before: string;
  text: string;
  after: string;
}

/** The server answers shorter queries with 400. */
export const MIN_SEARCH_LENGTH = 3;

/** Thrown when search is not running (the server answers 503), so the UI can say so. */
export class SearchUnavailableError extends Error {
  constructor() {
    super("Search is not available right now.");
    this.name = "SearchUnavailableError";
  }
}

export async function searchSubtitles(streamId: number, query: string, limit = 20): Promise<SubtitleHit[]> {
  const params = new URLSearchParams({ query, limit: String(limit) });
  try {
    const json = await apiFetchOptionalAuth<JsonValue>(`/streams/${streamId}/subtitles/search?${params}`, { method: "GET" });
    const res = fromJson(SearchStreamSubtitlesResponseSchema, json, JSON_READ_OPTIONS);
    return res.hits.map((h) => ({
      // int64 arrives as a bigint; milliseconds into a lecture fit a Number easily.
      time: Number(h.timestamp) / 1000,
      before: h.textPrev,
      text: h.text,
      after: h.textNext,
    }));
  } catch (err) {
    if (err instanceof ApiError && err.status === 503) throw new SearchUnavailableError();
    throw err;
  }
}
