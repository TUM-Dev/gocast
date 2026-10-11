import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { effectScope } from "vue";
import { describe, expect, it, vi } from "vitest";

import { RealtimeEventSchema, type RealtimeEvent } from "@/gen/server/apiv2_pb";
import { HIGHLIGHT_MS, useLiveEvents, type EventSource } from "./watch-admin-live";

function setup(onResync = vi.fn()) {
  let handler: (e: RealtimeEvent) => void = () => {};
  const unsubscribe = vi.fn();
  const source: EventSource = (h) => {
    handler = h;
    return unsubscribe;
  };
  const scope = effectScope();
  const live = scope.run(() => useLiveEvents(1, onResync, source))!;
  const send = (event: RealtimeEvent["event"]) => handler(create(RealtimeEventSchema, { event }));
  return { live, send, scope, unsubscribe, onResync };
}

describe("useLiveEvents", () => {
  it("reads the events of the shared source", () => {
    vi.useFakeTimers();
    const { live, send, scope, unsubscribe, onResync } = setup();
    send({ case: "viewers", value: { viewers: 12 } as never });
    send({ case: "live", value: { live: false } as never });
    send({ case: "streamReactionStats", value: { shares: [{ reaction: "👍", share: 1 }], windowSeconds: 30 } as never });
    send({ case: "streamReaction", value: { reaction: "👍" } as never });
    send({ case: "timeChanged", value: { end: timestampFromDate(new Date("2026-01-01T11:00:00Z")) } as never });
    send({ case: "resync", value: {} as never });
    expect(live.viewers.value).toBe(12);
    expect(live.live.value).toBe(false);
    expect(live.stats.value?.shares[0]?.reaction).toBe("👍");
    expect(live.lastReaction.value).toBe("👍");
    expect(live.end.value?.toISOString()).toBe("2026-01-01T11:00:00.000Z");
    expect(onResync).toHaveBeenCalledOnce();
    vi.advanceTimersByTime(HIGHLIGHT_MS);
    expect(live.lastReaction.value).toBe("");
    scope.stop();
    expect(unsubscribe).toHaveBeenCalled();
    vi.useRealTimers();
  });
});
