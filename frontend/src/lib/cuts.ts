/**
 * The cuts of the cut editor: stretches of the recording to remove, in seconds from its
 * start. Pure logic, so that what the editor shows is what the server accepts: cuts are
 * sorted and apart, inside the recording, and not shorter than {@link MIN_CUT}.
 */

export interface Cut {
  /** Names the cut while it is edited; the server does not know it. */
  id: number;
  start: number;
  end: number;
  /** Proposed from the silences and not applied until the person accepts it. */
  suggested: boolean;
}

/** The shortest cut. Anything shorter is a slip of the pointer. */
export const MIN_CUT = 0.5;
/** The least that has to remain, as the server requires. */
export const MIN_KEPT = 1;

/** What a suggestion leaves of a silence's edges, so speech is not clipped. */
export const SUGGESTION_PADDING = 1.5;
/** The shortest stretch worth suggesting once the padding is taken off. */
export const MIN_SUGGESTION = 5;

let lastId = 0;
export function nextCutId(): number {
  return ++lastId;
}

const clamp = (v: number, lo: number, hi: number) => Math.min(Math.max(v, lo), hi);

/**
 * Brings cuts into the shape the server accepts: clamped to the recording, sorted, with
 * overlapping or touching cuts joined and slivers dropped. A joined cut is only a
 * suggestion if all it was made of were. Suggestions and accepted cuts are normalised
 * separately by the callers that mix them, see {@link addCut}.
 */
export function normalizeCuts(cuts: readonly Cut[], duration: number): Cut[] {
  const sorted = cuts
    .map((c) => ({ ...c, start: clamp(c.start, 0, duration), end: clamp(c.end, 0, duration) }))
    .filter((c) => c.end - c.start >= MIN_CUT)
    .sort((a, b) => a.start - b.start || a.end - b.end);

  const out: Cut[] = [];
  for (const c of sorted) {
    const last = out[out.length - 1];
    if (last && c.start <= last.end && c.suggested === last.suggested) {
      last.end = Math.max(last.end, c.end);
    } else {
      out.push(c);
    }
  }
  return out;
}

/**
 * Adds an accepted cut, which takes over the stretch it covers: a suggestion it overlaps
 * is absorbed, an accepted cut it overlaps is joined to it. The start may be after the end,
 * as dragging backwards makes it.
 */
export function addCut(cuts: readonly Cut[], start: number, end: number, duration: number): Cut[] {
  const lo = clamp(Math.min(start, end), 0, duration);
  const hi = clamp(Math.max(start, end), 0, duration);
  if (hi - lo < MIN_CUT) return [...cuts];
  const accepted = cuts.filter((c) => !c.suggested);
  const suggestions = cuts.filter((c) => c.suggested);
  const merged = normalizeCuts([...accepted, { id: nextCutId(), start: lo, end: hi, suggested: false }], duration);
  // A suggestion the new cut touches is gone, or what is left of it at either side stays.
  const rest = suggestions.flatMap((s) => subtract(s, merged));
  return [...merged, ...rest].sort((a, b) => a.start - b.start);
}

/** The parts of a suggestion that no accepted cut covers. */
function subtract(s: Cut, accepted: readonly Cut[]): Cut[] {
  let parts: Cut[] = [s];
  for (const a of accepted) {
    parts = parts.flatMap((p) => {
      if (a.end <= p.start || a.start >= p.end) return [p];
      const pieces: Cut[] = [];
      if (a.start > p.start) pieces.push({ ...p, end: a.start });
      if (a.end < p.end) pieces.push({ ...p, id: pieces.length ? nextCutId() : p.id, start: a.end });
      return pieces;
    });
  }
  return parts.filter((p) => p.end - p.start >= MIN_CUT);
}

export function removeCut(cuts: readonly Cut[], id: number): Cut[] {
  return cuts.filter((c) => c.id !== id);
}

/** Turns a suggestion into an accepted cut, joined to the accepted cuts it touches. */
export function acceptCut(cuts: readonly Cut[], id: number, duration: number): Cut[] {
  const s = cuts.find((c) => c.id === id);
  if (!s) return [...cuts];
  return addCut(removeCut(cuts, id), s.start, s.end, duration);
}

export function acceptAll(cuts: readonly Cut[], duration: number): Cut[] {
  return normalizeCuts(
    cuts.map((c) => ({ ...c, suggested: false })),
    duration,
  );
}

/** Moves one edge of a cut, keeping it apart from its neighbours and at least MIN_CUT long. */
export function resizeCut(
  cuts: readonly Cut[],
  id: number,
  edge: "start" | "end",
  to: number,
  duration: number,
): Cut[] {
  const i = cuts.findIndex((c) => c.id === id);
  if (i < 0) return [...cuts];
  const c = cuts[i];
  // Only the cuts of the same kind are neighbours: a suggestion and an accepted cut may
  // lie over each other while the person decides, and the accepted one wins at submit.
  const same = cuts.filter((o) => o.suggested === c.suggested);
  const j = same.findIndex((o) => o.id === id);
  const before = same[j - 1]?.end ?? 0;
  const after = same[j + 1]?.start ?? duration;
  const next =
    edge === "start"
      ? { ...c, start: clamp(to, before, c.end - MIN_CUT) }
      : { ...c, end: clamp(to, c.start + MIN_CUT, after) };
  return cuts.map((o) => (o.id === id ? next : o));
}

/**
 * Cuts proposed from the detected silences. A silence is cut with a little left at either
 * edge, so the first word after it and the last before it stay whole; the edges of the
 * recording need no padding. Silences too short for that to leave anything are skipped.
 */
export function suggestCuts(silences: readonly { start: number; end: number }[], duration: number): Cut[] {
  const out: Cut[] = [];
  for (const s of silences) {
    const start = s.start <= 0 ? 0 : s.start + SUGGESTION_PADDING;
    const end = s.end >= duration ? duration : s.end - SUGGESTION_PADDING;
    if (end - start >= MIN_SUGGESTION) out.push({ id: nextCutId(), start, end, suggested: true });
  }
  return normalizeCuts(out, duration);
}

/** The cuts that are applied: the accepted ones. */
export function appliedCuts(cuts: readonly Cut[]): Cut[] {
  return cuts.filter((c) => !c.suggested);
}

export function removedSeconds(cuts: readonly Cut[]): number {
  return appliedCuts(cuts).reduce((sum, c) => sum + (c.end - c.start), 0);
}

export function keptSeconds(cuts: readonly Cut[], duration: number): number {
  return Math.max(0, duration - removedSeconds(cuts));
}

/** Whether the applied cuts leave enough of the recording for the server to take them. */
export function leavesEnough(cuts: readonly Cut[], duration: number): boolean {
  return keptSeconds(cuts, duration) >= MIN_KEPT;
}

/** The applied cut a time lies in, if any. The end belongs to what is kept. */
export function cutAt(cuts: readonly Cut[], time: number): Cut | undefined {
  return appliedCuts(cuts).find((c) => time >= c.start && time < c.end);
}

/** Where the time lies once the applied cuts are out of the recording. */
export function timeAfterCuts(cuts: readonly Cut[], time: number): number {
  let removed = 0;
  for (const c of appliedCuts(cuts)) removed += Math.max(0, Math.min(c.end, time) - c.start);
  return Math.max(0, time - removed);
}

/** The ranges as the server takes them. */
export function toRanges(cuts: readonly Cut[]): { start: number; end: number }[] {
  return appliedCuts(cuts).map((c) => ({ start: c.start, end: c.end }));
}

/* The timeline. */

/** The time at a position of a timeline of `width` pixels that shows `duration`. */
export function timeAtX(x: number, width: number, duration: number): number {
  return width <= 0 ? 0 : clamp((x / width) * duration, 0, duration);
}

export function xAtTime(time: number, width: number, duration: number): number {
  return duration <= 0 ? 0 : (clamp(time, 0, duration) / duration) * width;
}

/**
 * One height per pixel column: the loudest bucket that column covers, 0 to 1. A column
 * covering several buckets shows the peak, as a quiet average would hide speech.
 */
export function columnLevels(peaks: ArrayLike<number>, bucketSeconds: number, duration: number, columns: number): number[] {
  const out = new Array<number>(Math.max(0, columns)).fill(0);
  if (columns <= 0 || duration <= 0 || peaks.length === 0) return out;
  for (let x = 0; x < columns; x++) {
    const from = Math.floor(((x / columns) * duration) / bucketSeconds);
    const to = Math.max(from + 1, Math.ceil((((x + 1) / columns) * duration) / bucketSeconds));
    let peak = 0;
    for (let i = from; i < Math.min(to, peaks.length); i++) peak = Math.max(peak, peaks[i]);
    out[x] = peak / 255;
  }
  return out;
}

/** "4:02", "1:04:02", or with tenths for the precision a cut is made at. */
export function formatTime(seconds: number, tenths = false): string {
  const t = Math.max(0, seconds);
  const whole = Math.floor(t);
  const h = Math.floor(whole / 3600);
  const m = Math.floor((whole % 3600) / 60);
  const s = String(whole % 60).padStart(2, "0");
  const frac = tenths ? `.${Math.floor((t - whole) * 10)}` : "";
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${s}${frac}` : `${m}:${s}${frac}`;
}
