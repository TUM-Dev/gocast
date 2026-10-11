/**
 * The realtime events the live management page reads.
 *
 * The page does not own a socket. It reads RealtimeEvents from an {@link EventSource},
 * which the shell provides under {@link LIVE_EVENT_SOURCE} (feed it from the page's
 * shared socket: `useStreamRealtime(id).subscribe`). Without one, it falls back to a
 * connection of its own from lib/realtime -- a second socket per tab, which is why the
 * shared one is preferred.
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { inject, onScopeDispose, ref, type InjectionKey, type Ref } from "vue";

import type { RealtimeEvent, StreamReactionStatsEvent } from "@/gen/server/apiv2_pb";
import { RealtimeConnection, type RealtimeStatus } from "./realtime";

/** Subscribes to every event of the stream; returns the unsubscribe. */
export type EventSource = (handler: (event: RealtimeEvent) => void) => () => void;

export const LIVE_EVENT_SOURCE: InjectionKey<EventSource> = Symbol("live-event-source");

export interface LiveEvents {
  status: Ref<RealtimeStatus>;
  viewers: Ref<number | undefined>;
  live: Ref<boolean | undefined>;
  stats: Ref<StreamReactionStatsEvent | undefined>;
  /** The newest reaction, for a moment: the emoji to highlight. */
  lastReaction: Ref<string>;
  end: Ref<Date | undefined>;
  /** Feed one event; what the subscription calls. Exposed for the shell and tests. */
  apply(event: RealtimeEvent): void;
  close(): void;
}

/** How long a single reaction stays highlighted. */
export const HIGHLIGHT_MS = 1000;

/**
 * @param onResync called when state may have been missed -- on a ResyncEvent, and (with
 *   the fallback socket) on every (re)subscribe: fetch the stream again.
 * @param source overrides the injected source.
 */
export function useLiveEvents(streamId: number, onResync?: () => void, source?: EventSource): LiveEvents {
  const status = ref<RealtimeStatus>("closed");
  const viewers = ref<number | undefined>();
  const live = ref<boolean | undefined>();
  const stats = ref<StreamReactionStatsEvent | undefined>();
  const lastReaction = ref("");
  const end = ref<Date | undefined>();
  let highlight: ReturnType<typeof setTimeout> | undefined;

  function apply(event: RealtimeEvent): void {
    const e = event.event;
    switch (e.case) {
      case "viewers":
        viewers.value = e.value.viewers;
        break;
      case "live":
        live.value = e.value.live;
        break;
      case "streamReactionStats":
        stats.value = e.value;
        break;
      case "streamReaction":
        lastReaction.value = e.value.reaction;
        clearTimeout(highlight);
        highlight = setTimeout(() => (lastReaction.value = ""), HIGHLIGHT_MS);
        break;
      case "timeChanged":
        if (e.value.end) end.value = timestampDate(e.value.end);
        break;
      case "resync":
        onResync?.();
        break;
    }
  }

  const provided = source ?? inject(LIVE_EVENT_SOURCE, undefined);
  let stop: () => void;
  if (provided) {
    status.value = "open";
    stop = provided(apply);
  } else {
    const connection = new RealtimeConnection({
      streamId,
      onStatus: (s) => (status.value = s),
      onEvent: apply,
      // The first subscribe is a resync too, but the page has just loaded: only
      // reconnects (and ResyncEvents, above) need a fetch.
      onResync: undefined,
    });
    connection.connect();
    stop = () => connection.close();
  }

  function close(): void {
    clearTimeout(highlight);
    stop();
  }
  onScopeDispose(close);

  return { status, viewers, live, stats, lastReaction, end, apply, close };
}
