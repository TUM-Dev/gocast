import { fromJson, type JsonValue } from "@bufbuild/protobuf";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RealtimeEventSchema } from "@/gen/server/apiv2_pb";
import type { RealtimeOptions } from "@/lib/realtime";
import { useStreamRealtime } from "./streamRealtime";

const { connections } = vi.hoisted(() => ({
  connections: [] as { options: RealtimeOptions; connect: () => void; close: () => void }[],
}));

// The connection's own behaviour is realtime.test.ts's; here it is only counted, and
// driven by hand.
vi.mock("@/lib/realtime", () => ({
  CLOSE_UNAUTHORIZED: 4401,
  RealtimeConnection: class {
    connect = vi.fn();
    close = vi.fn();
    constructor(readonly options: RealtimeOptions) {
      connections.push(this);
    }
  },
}));

const event = (json: object) => fromJson(RealtimeEventSchema, { streamId: 7, ...json } as JsonValue);

beforeEach(() => {
  setActivePinia(createPinia());
  connections.length = 0;
});

describe("one socket per tab", () => {
  it("opens a single connection however many consumers listen", () => {
    const rt = useStreamRealtime(7);

    rt.subscribe({});
    rt.subscribe({});
    // A second store handle, as another component would get it.
    useStreamRealtime(7).subscribe({});

    expect(connections).toHaveLength(1);
    expect(connections[0]!.options.streamId).toBe(7);
    expect(connections[0]!.connect).toHaveBeenCalledTimes(1);
  });

  it("closes the connection when the last consumer unsubscribes, not before", () => {
    const rt = useStreamRealtime(7);
    const first = rt.subscribe({});
    const second = rt.subscribe({});

    first();
    first(); // safe to repeat
    expect(connections[0]!.close).not.toHaveBeenCalled();

    second();
    expect(connections[0]!.close).toHaveBeenCalledTimes(1);
    expect(rt.status).toBe("closed");
  });

  it("opens a fresh connection when a consumer returns after the last left", () => {
    const rt = useStreamRealtime(7);
    rt.subscribe({})();
    rt.subscribe({});

    expect(connections).toHaveLength(2);
  });

  it("keeps a connection per stream", () => {
    useStreamRealtime(7).subscribe({});
    useStreamRealtime(8).subscribe({});

    expect(connections.map((c) => c.options.streamId)).toEqual([7, 8]);
  });
});

describe("what consumers hear", () => {
  it("passes every event to every consumer, after applying its own", () => {
    const rt = useStreamRealtime(7);
    const a = vi.fn();
    const b = vi.fn();
    rt.subscribe({ onEvent: a });
    rt.subscribe({ onEvent: b });

    connections[0]!.options.onEvent!(event({ viewers: { viewers: 42 } }));
    connections[0]!.options.onEvent!(event({ message: { message: { id: 1 } } }));

    expect(a).toHaveBeenCalledTimes(2);
    expect(b).toHaveBeenCalledTimes(2);
    expect(rt.viewers).toBe(42);
  });

  it("stops telling a consumer that unsubscribed", () => {
    const rt = useStreamRealtime(7);
    const gone = vi.fn();
    const kept = vi.fn();
    const off = rt.subscribe({ onEvent: gone });
    rt.subscribe({ onEvent: kept });

    off();
    connections[0]!.options.onEvent!(event({ live: { live: true } }));

    expect(gone).not.toHaveBeenCalled();
    expect(kept).toHaveBeenCalledTimes(1);
  });

  it("tracks viewers, live state, title and description", () => {
    const rt = useStreamRealtime(7);
    rt.subscribe({});
    const { onEvent } = connections[0]!.options;

    onEvent!(event({ viewers: { viewers: 3 } }));
    onEvent!(event({ live: { live: true } }));
    onEvent!(event({ title: { title: "Hopfen" } }));
    onEvent!(event({ description: { html: "<p>Malz</p>" } }));

    expect(rt.viewers).toBe(3);
    expect(rt.live).toBe(true);
    expect(rt.title).toBe("Hopfen");
    expect(rt.descriptionHtml).toBe("<p>Malz</p>");
  });

  it("forgets the stream's facts when the socket closes", () => {
    const rt = useStreamRealtime(7);
    const off = rt.subscribe({});
    connections[0]!.options.onEvent!(event({ viewers: { viewers: 3 } }));
    connections[0]!.options.onEvent!(event({ live: { live: true } }));

    off();

    expect(rt.viewers).toBe(0);
    expect(rt.live).toBeUndefined();
  });

  it("asks each consumer to load afresh when the socket subscribes or resyncs", () => {
    const rt = useStreamRealtime(7);
    const a = vi.fn();
    const b = vi.fn();
    rt.subscribe({ onResync: a });
    rt.subscribe({ onResync: b });
    expect(a).not.toHaveBeenCalled(); // nothing is subscribed yet

    connections[0]!.options.onResync!();

    expect(a).toHaveBeenCalledTimes(1);
    expect(b).toHaveBeenCalledTimes(1);
  });

  it("asks a consumer that joins a subscribed socket to load straight away", () => {
    const rt = useStreamRealtime(7);
    rt.subscribe({});
    connections[0]!.options.onResync!();

    const late = vi.fn();
    rt.subscribe({ onResync: late });

    expect(late).toHaveBeenCalledTimes(1);
    expect(connections).toHaveLength(1);
  });
});

describe("the connection's state", () => {
  it("follows status and close codes", () => {
    const rt = useStreamRealtime(7);
    rt.subscribe({});
    const { options } = connections[0]!;

    options.onStatus!("connecting");
    expect(rt.status).toBe("connecting");

    options.onClose!(4401, "token expired");
    expect(rt.lastCloseCode).toBe(4401);

    options.onStatus!("open");
    expect(rt.status).toBe("open");
    expect(rt.lastCloseCode).toBeUndefined();
  });

  it("tells every consumer of a refusal, and a late one straight away", () => {
    const rt = useStreamRealtime(7);
    const a = vi.fn();
    rt.subscribe({ onFatal: a });

    connections[0]!.options.onFatal!(4403, "not eligible");
    const late = vi.fn();
    rt.subscribe({ onFatal: late });

    expect(rt.fatalCode).toBe(4403);
    expect(rt.sessionExpired).toBe(false);
    expect(a).toHaveBeenCalledWith(4403, "not eligible");
    expect(late).toHaveBeenCalledWith(4403, "not eligible");
  });

  it("calls a final 4401 an expired session", () => {
    const rt = useStreamRealtime(7);
    rt.subscribe({});

    connections[0]!.options.onFatal!(4401, "expired");

    expect(rt.sessionExpired).toBe(true);
  });

  it("starts clean when the socket is opened again after a refusal", () => {
    const rt = useStreamRealtime(7);
    const off = rt.subscribe({});
    connections[0]!.options.onFatal!(4403, "x");
    off();

    // The last unsubscribe closed it; the next subscriber is not told of the old refusal.
    const fresh = vi.fn();
    rt.subscribe({ onFatal: fresh });
    expect(fresh).not.toHaveBeenCalled();
    expect(rt.fatalCode).toBeUndefined();
  });
});
