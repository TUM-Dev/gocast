import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RealtimeConnection, backoffDelay, type RealtimeOptions } from "./realtime";

/** A WebSocket the test drives by hand. */
class FakeSocket {
  static instances: FakeSocket[] = [];

  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: ((e: { code: number; reason: string }) => void) | null = null;
  sent: string[] = [];
  closedWith: number | undefined;

  constructor(readonly url: string) {
    FakeSocket.instances.push(this);
  }

  send(data: string): void {
    this.sent.push(data);
  }

  close(code?: number): void {
    this.closedWith = code;
  }

  // Test controls.
  open(): void {
    this.onopen?.();
  }
  receive(event: unknown): void {
    this.onmessage?.({ data: JSON.stringify(event) });
  }
  drop(code = 1006, reason = ""): void {
    this.onclose?.({ code, reason });
  }
}

const latest = () => FakeSocket.instances.at(-1)!;

/** Lets the token promise and the socket construction behind it settle. */
const settle = () => vi.advanceTimersByTimeAsync(0);

function connect(overrides: Partial<RealtimeOptions> = {}) {
  const options = {
    streamId: 7,
    url: "ws://test/api/v2/realtime?stream=7",
    getToken: vi.fn().mockResolvedValue("tok"),
    onEvent: vi.fn(),
    onResync: vi.fn(),
    onStatus: vi.fn(),
    onFatal: vi.fn(),
    random: () => 0.5,
    ...overrides,
  };
  const conn = new RealtimeConnection(options);
  conn.connect();
  return { conn, options };
}

beforeEach(() => {
  vi.useFakeTimers();
  FakeSocket.instances = [];
  vi.stubGlobal("WebSocket", FakeSocket);
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("RealtimeConnection", () => {
  it("sends the bearer token as its first frame", async () => {
    connect();
    await settle();

    latest().open();

    expect(latest().url).toBe("ws://test/api/v2/realtime?stream=7");
    expect(latest().sent).toEqual([JSON.stringify({ token: "tok" })]);
  });

  it("sends an empty credentials frame when nobody is signed in", async () => {
    connect({ getToken: vi.fn().mockResolvedValue(null) });
    await settle();

    latest().open();

    expect(latest().sent).toEqual(["{}"]);
  });

  it("parses frames into typed events and resyncs once subscribed", async () => {
    const { conn, options } = connect();
    const titles: string[] = [];
    conn.on("title", (value) => titles.push(value.title));
    await settle();
    latest().open();

    latest().receive({ streamId: 7, audience: "AUDIENCE_ALL", viewers: { viewers: 3 } });
    latest().receive({
      streamId: 7,
      audience: "AUDIENCE_ALL",
      title: { title: "Hopfen" },
      unknownField: 1,
    });

    expect(options.onEvent).toHaveBeenCalledTimes(2);
    const first = vi.mocked(options.onEvent!).mock.calls[0]![0];
    expect(first.event).toMatchObject({ case: "viewers", value: { viewers: 3 } });
    expect(titles).toEqual(["Hopfen"]);
    // Once, on the first frame: from then on nothing can be missed.
    expect(options.onResync).toHaveBeenCalledTimes(1);
    expect(options.onStatus).toHaveBeenLastCalledWith("open");
  });

  it("resyncs when the server says so", async () => {
    const { options } = connect();
    await settle();
    latest().receive({ viewers: {} });

    latest().receive({ resync: { reason: "fell behind" } });

    expect(options.onResync).toHaveBeenCalledTimes(2);
  });

  it("ignores a frame it cannot read", async () => {
    const { options } = connect();
    await settle();

    latest().onmessage?.({ data: "not json" });

    expect(options.onEvent).not.toHaveBeenCalled();
  });

  it("reconnects with backoff, re-minting the token, and resyncs after", async () => {
    const { options } = connect();
    await settle();
    latest().receive({ viewers: {} });

    latest().drop();
    expect(FakeSocket.instances).toHaveLength(1);

    // attempt 0: ceiling 1 s, half fixed plus half of 0.5 random = 750 ms
    await vi.advanceTimersByTimeAsync(749);
    expect(FakeSocket.instances).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(FakeSocket.instances).toHaveLength(2);
    expect(options.getToken).toHaveBeenCalledTimes(2);

    latest().receive({ viewers: {} });
    expect(options.onResync).toHaveBeenCalledTimes(2);
  });

  it("backs off further each time a connection fails before subscribing", async () => {
    connect();
    await settle();

    latest().drop(); // attempt 0: 750 ms
    await vi.advanceTimersByTimeAsync(750);
    latest().drop(); // attempt 1: 1500 ms
    await vi.advanceTimersByTimeAsync(1499);
    expect(FakeSocket.instances).toHaveLength(2);
    await vi.advanceTimersByTimeAsync(1);
    expect(FakeSocket.instances).toHaveLength(3);
  });

  it("retries a refused token, but gives up on a refused stream", async () => {
    const { options } = connect();
    await settle();

    latest().drop(4401, "token is expired");
    await vi.advanceTimersByTimeAsync(1000);
    expect(FakeSocket.instances).toHaveLength(2);

    latest().drop(4403, "not allowed");
    await vi.advanceTimersByTimeAsync(60_000);
    expect(FakeSocket.instances).toHaveLength(2);
    expect(options.onFatal).toHaveBeenCalledWith(4403, "not allowed");
  });

  it("stays closed after close()", async () => {
    const { conn } = connect();
    await settle();
    const socket = latest();

    conn.close();
    socket.drop(1000);
    await vi.advanceTimersByTimeAsync(60_000);

    expect(socket.closedWith).toBe(1000);
    expect(FakeSocket.instances).toHaveLength(1);
  });

  it("retries when no token could be minted", async () => {
    const getToken = vi.fn().mockRejectedValueOnce(new Error("offline")).mockResolvedValue("tok");
    connect({ getToken });
    await settle();
    expect(FakeSocket.instances).toHaveLength(0);

    await vi.advanceTimersByTimeAsync(750);
    expect(FakeSocket.instances).toHaveLength(1);
  });
});

describe("backoffDelay", () => {
  it("doubles from one second and stops at thirty", () => {
    expect(backoffDelay(0, () => 0)).toBe(500);
    expect(backoffDelay(0, () => 0.5)).toBe(750);
    expect(backoffDelay(3, () => 0)).toBe(4000);
    expect(backoffDelay(10, () => 1)).toBe(30_000);
  });
});
