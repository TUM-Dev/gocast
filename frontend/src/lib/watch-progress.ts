import { create } from "@bufbuild/protobuf";

import { StreamProgressSchema, UpdateProgressRequestSchema } from "@/gen/server/apiv2_pb";
import { apiPatchMessage } from "./api";

/** Saves the viewer's position, as a fraction of the recording from 0 to 1. */
export async function saveProgress(streamId: number, progress: number): Promise<void> {
  await apiPatchMessage(
    UpdateProgressRequestSchema,
    StreamProgressSchema,
    `/progress/${streamId}`,
    // No `watched`: absent means "leave the flag alone", while an explicit false would unmark.
    create(UpdateProgressRequestSchema, { streamId, progress }),
  );
}

/**
 * Marks the lecture watched (what the course page's eye icon reads). The plain progress
 * saves above leave `watched` out of the request, which the server reads as "keep the
 * flag", so a late pause report cannot undo this.
 */
export async function markWatched(streamId: number): Promise<void> {
  await apiPatchMessage(
    UpdateProgressRequestSchema,
    StreamProgressSchema,
    `/progress/${streamId}`,
    create(UpdateProgressRequestSchema, { streamId, progress: 1, watched: true }),
  );
}

/**
 * Calls `mark` the first time `fire` runs and never again for this marker: a viewer who
 * scrubs back and reaches the end twice is still one page view. A failed attempt is
 * forgotten, so the next end gets another try.
 */
export function createWatchedMarker(mark: () => Promise<void>): { fire(): void } {
  let done = false;
  return {
    fire() {
      if (done) return;
      done = true;
      mark().catch(() => {
        done = false;
      });
    },
  };
}

/** Playback must move this far (seconds) from where it started before progress is saved. */
const MIN_ADVANCE_SECONDS = 3;

export interface ProgressReporter {
  onPlay(): void;
  onPause(): void;
  onEnded(): void;
  dispose(): void;
}

export interface ProgressReporterOptions {
  /** The current position as a fraction, or undefined while it is not known. */
  fraction(): number | undefined;
  save(progress: number): Promise<void>;
  /** Milliseconds between reports while playing; the old player's 10 s. */
  intervalMs?: number;
  /**
   * How far (as a fraction) the position must move from where playback started before
   * anything is reported. Opening a lecture must not save a position: a finished one
   * restarts at 0:00, and reporting that would overwrite the viewer's progress with ~0.
   */
  minAdvance?: () => number;
}

/**
 * Reports watch progress the way the old player did (TUMLiveVjs.ts watchProgress):
 * every 10 s while playing, immediately on pause (which is also what a seek-and-stop or
 * leaving by pausing produces), nothing once ended. Nothing at all is reported until the
 * position has moved from where playback started (see `minAdvance`). A failed report doubles the interval
 * -- binary exponential backoff, so a struggling server is not hammered by every viewer.
 * It deliberately stays doubled after later successes, as before.
 *
 * Timers are `setTimeout` chains rather than an interval so a slow report cannot pile
 * up behind the next.
 */
export function createProgressReporter(options: ProgressReporterOptions): ProgressReporter {
  let interval = options.intervalMs ?? 10_000;
  let timer: ReturnType<typeof setTimeout> | undefined;

  // Where the position was when playback first started; reports wait until it moves.
  let baseline: number | undefined;
  let engaged = false;

  const current = () => {
    const progress = options.fraction();
    return progress === undefined || !Number.isFinite(progress) ? undefined : progress;
  };
  const report = () => {
    const progress = current();
    if (progress === undefined) return;
    if (!engaged) {
      if (baseline === undefined) baseline = progress;
      if (Math.abs(progress - baseline) <= 0 || Math.abs(progress - baseline) < (options.minAdvance?.() ?? 0)) return;
      engaged = true;
    }
    options.save(Math.min(1, Math.max(0, progress))).catch(() => {
      interval *= 2;
    });
  };
  const stop = () => {
    clearTimeout(timer);
    timer = undefined;
  };
  const schedule = () => {
    timer = setTimeout(() => {
      report();
      schedule();
    }, interval);
  };

  return {
    onPlay() {
      if (!engaged && baseline === undefined) baseline = current();
      stop();
      schedule();
    },
    onPause() {
      stop();
      report();
    },
    onEnded: stop,
    dispose: stop,
  };
}

/** What {@link attachProgressReporting} needs of a player: both layouts' controllers have it. */
export interface ReportablePlayer {
  getCurrentTime(): number;
  getDuration(): number;
  toRecordingTime(clipTime: number): number;
  on(event: "play" | "pause" | "ended", fn: () => void): () => void;
}

/**
 * Reports the position of `player` as the single player and the split view both do:
 * the fraction of the recording (the clip's start is added back when the recording's
 * length is not known), on the reporter's cadence. `onEnded` runs when playback reaches
 * the end of the recording or of its clip. Returns the function that detaches it.
 */
export function attachProgressReporting(
  player: ReportablePlayer,
  options: { streamId: number; recordingDuration?: number; clipStart?: number; onEnded?: () => void },
): () => void {
  const total = () => options.recordingDuration ?? player.getDuration() + (options.clipStart ?? 0);
  const reporter = createProgressReporter({
    fraction() {
      const t = total();
      if (!Number.isFinite(t) || t <= 0) return undefined;
      return player.toRecordingTime(player.getCurrentTime()) / t;
    },
    save: (progress) => saveProgress(options.streamId, progress),
    minAdvance: () => MIN_ADVANCE_SECONDS / total(),
  });
  const offs = [
    player.on("play", reporter.onPlay),
    player.on("pause", reporter.onPause),
    player.on("ended", () => {
      reporter.onEnded();
      options.onEnded?.();
    }),
  ];
  return () => {
    offs.forEach((off) => off());
    reporter.dispose();
  };
}
