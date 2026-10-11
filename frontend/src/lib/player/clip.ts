import type { PlayerLike } from "./types";
import { timeRanges } from "./types";

/**
 * Play only part of a recording. Used for a stream's start and end offsets: the server-side
 * `wowzaplaystart` offsets do not work on signed streams (nothing reads them), so the
 * player does the clipping itself.
 */
export interface Clip {
  /** Seconds into the recording where playback begins. */
  start: number;
  /** Seconds into the recording where it stops; the end of the recording if omitted. */
  end?: number;
}

export interface ClipHandle {
  /** The recording's own time for a time on the clip's clock. */
  toRecordingTime(clipTime: number): number;
  /** The clip's clock for the recording's time. */
  toClipTime(recordingTime: number): number;
  /** Removes the clip: the player is back to the whole recording. */
  dispose(): void;
}

// timeupdate fires about four times a second, so playback can run this far past the end
// before the pause. Stopping slightly early is not worth a polling timer, and the
// position is put back exactly on the end afterwards.
const END_TOLERANCE = 0.05;

type Methods = "currentTime" | "duration" | "buffered" | "seekable" | "ended";

/**
 * Makes the player behave as if the clip were the whole recording: `currentTime()` and
 * `duration()` -- and so the seek bar, the time displays and every seek -- are relative
 * to the clip, and playback pauses at its end.
 *
 * The alternative, leaving the player on the whole recording and only guarding the
 * ends, would show a seek bar that spans a lecture to play five minutes of it, with a
 * time display that starts at 1:12:00. Presenting the clip as the media costs a thin
 * facade over five player methods, which video.js' own components all go through
 * (`player.currentTime()` etc.), so nothing needs patching inside video.js. The facade is
 * installed per instance and removed again by `dispose()`.
 *
 * What does not follow: the sprite-thumbnail plugin maps the pointer to a time by
 * `player.duration()` with no way to add an offset, so previews are wrong for a clip
 * that does not start at 0 (VideoPlayer.vue switches them off then). Everything that
 * talks to the player from outside -- the parent, the hotkeys, the events -- sees clip
 * time only; {@link ClipHandle.toRecordingTime} converts when it matters.
 */
export function installClip(player: PlayerLike, clip: Clip): ClipHandle {
  const start = Math.max(0, clip.start);
  const target = player as unknown as Record<Methods, (...args: unknown[]) => unknown>;
  // video.js keeps these on the prototype, so deleting our instance property restores
  // them; a player that had its own would be put back exactly as it was.
  const own = new Map<Methods, PropertyDescriptor | undefined>(
    (["currentTime", "duration", "buffered", "seekable", "ended"] as const).map((name) => [
      name,
      Object.getOwnPropertyDescriptor(player, name),
    ]),
  );
  const raw = {
    currentTime: target.currentTime.bind(player) as (seconds?: number) => number,
    duration: target.duration.bind(player) as (seconds?: number) => number,
    buffered: target.buffered.bind(player) as () => TimeRanges,
    seekable: target.seekable.bind(player) as () => TimeRanges,
    ended: target.ended.bind(player) as () => boolean,
  };
  let reachedEnd = false;

  /** Where, on the recording's clock, the clip ends. */
  const endTime = (): number => {
    const whole = raw.duration();
    const wholeKnown = Number.isFinite(whole) && whole > 0;
    if (clip.end === undefined) return wholeKnown ? whole : NaN;
    return wholeKnown ? Math.min(clip.end, whole) : clip.end;
  };
  const length = (): number => {
    const end = endTime();
    return Number.isFinite(end) ? Math.max(0, end - start) : NaN;
  };
  const shift = (ranges: TimeRanges): TimeRanges => {
    const end = endTime();
    const out: [number, number][] = [];
    for (let i = 0; i < ranges.length; i++) {
      const from = Math.max(ranges.start(i), start);
      const to = Number.isFinite(end) ? Math.min(ranges.end(i), end) : ranges.end(i);
      if (to > from) out.push([from - start, to - start]);
    }
    return timeRanges(...out);
  };

  // `duration(seconds)` is video.js' own setter, which stores the length the tech
  // reported and fires `durationchange`; it must keep working or the displays never update.
  target.duration = (seconds?: unknown) => (seconds === undefined ? length() : raw.duration(seconds as number));
  target.currentTime = (seconds?: unknown) => {
    if (seconds === undefined) {
      const len = length();
      const t = Math.max(0, raw.currentTime() - start);
      return Number.isFinite(len) ? Math.min(t, len) : t;
    }
    const len = length();
    const wanted = Math.max(0, seconds as number);
    reachedEnd = false;
    return raw.currentTime(start + (Number.isFinite(len) ? Math.min(wanted, len) : wanted));
  };
  target.buffered = () => shift(raw.buffered());
  target.seekable = () => shift(raw.seekable());
  target.ended = () => reachedEnd || raw.ended();

  const stopAtEnd = () => {
    const end = endTime();
    if (!Number.isFinite(end)) return;
    const now = raw.currentTime();
    if (now >= end - END_TOLERANCE) {
      if (reachedEnd) return;
      reachedEnd = true;
      player.pause();
      raw.currentTime(end);
      player.trigger("ended");
    } else if (now < start - 0.5) {
      // Something moved the playhead before the clip (the source's own start, a
      // quality switch); the clip's start is the earliest a viewer may see.
      raw.currentTime(start);
    }
  };
  // Playing again from the end replays the clip rather than the last frame.
  const restartIfAtEnd = () => {
    const end = endTime();
    if (reachedEnd || (Number.isFinite(end) && raw.currentTime() >= end - END_TOLERANCE)) {
      reachedEnd = false;
      raw.currentTime(start);
    }
  };
  const toStart = () => {
    if (raw.currentTime() < start) raw.currentTime(start);
  };

  player.on("timeupdate", stopAtEnd);
  player.on("play", restartIfAtEnd);
  player.on("loadedmetadata", toStart);
  // Before metadata video.js queues the seek and applies it when playback can start.
  if (start > 0) raw.currentTime(start);
  // The displays last read the whole recording's length.
  player.trigger("durationchange");

  return {
    toRecordingTime: (t) => start + t,
    toClipTime: (t) => Math.max(0, t - start),
    dispose() {
      player.off("timeupdate", stopAtEnd);
      player.off("play", restartIfAtEnd);
      player.off("loadedmetadata", toStart);
      for (const [name, descriptor] of own) {
        if (descriptor) Object.defineProperty(player, name, descriptor);
        else delete (target as Partial<typeof target>)[name];
      }
      player.trigger("durationchange");
    },
  };
}
