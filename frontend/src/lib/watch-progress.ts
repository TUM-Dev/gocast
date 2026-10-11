import { create } from "@bufbuild/protobuf";

import { StreamProgressSchema, UpdateProgressRequestSchema } from "@/gen/server/apiv2_pb";
import { apiPatchMessage } from "./api";

/** Saves the viewer's position, as a fraction of the recording from 0 to 1. */
export async function saveProgress(streamId: number, progress: number): Promise<void> {
  await apiPatchMessage(
    UpdateProgressRequestSchema,
    StreamProgressSchema,
    `/progress/${streamId}`,
    create(UpdateProgressRequestSchema, { streamId, progress, watched: false }),
  );
}

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
}

/**
 * Reports watch progress the way the old player did (TUMLiveVjs.ts watchProgress):
 * every 10 s while playing, immediately on pause (which is also what a seek-and-stop or
 * leaving by pausing produces), nothing once ended. A failed report doubles the interval
 * -- binary exponential backoff, so a struggling server is not hammered by every viewer.
 * It deliberately stays doubled after later successes, as before.
 *
 * Timers are `setTimeout` chains rather than an interval so a slow report cannot pile
 * up behind the next.
 */
export function createProgressReporter(options: ProgressReporterOptions): ProgressReporter {
  let interval = options.intervalMs ?? 10_000;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const report = () => {
    const progress = options.fraction();
    if (progress === undefined || !Number.isFinite(progress)) return;
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
