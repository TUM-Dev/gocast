/**
 * The realtime socket: one stream's RealtimeEvents, pushed by the server.
 *
 * GET /api/v2/realtime?stream=<id> is a websocket the server only writes to, one
 * protojson-encoded RealtimeEvent per text frame. A connection authenticates with
 * the session cookie when the browser has one; otherwise its first frame must be
 * `{"token":"…"}`, or `{}` to watch anonymously. This sends that frame always: the
 * server ignores it when the cookie already answered.
 *
 * The socket only says what changed, so a client that may have missed something —
 * after any reconnect, or when told so with a ResyncEvent — fetches its state again.
 * That is the `onResync` callback.
 */

import { fromJson, type JsonValue } from "@bufbuild/protobuf";

import { RealtimeEventSchema, type RealtimeEvent } from "@/gen/server/apiv2_pb";
import { freshTokenOrNull } from "./api";

export type { RealtimeEvent };

/** The kinds of event, as the `case` of RealtimeEvent's oneof. */
export type RealtimeEventCase = NonNullable<RealtimeEvent["event"]["case"]>;

/** The payload of one kind of event. */
export type RealtimeEventValue<C extends RealtimeEventCase> = Extract<
  RealtimeEvent["event"],
  { case: C }
>["value"];

export type RealtimeStatus = "connecting" | "open" | "closed";

/**
 * Close codes the server sends: 4000 + the HTTP status the same failure gets over
 * REST. The rest will not change by trying again.
 */
const FATAL_CLOSE_CODES = new Set([4400, 4403, 4404]);

/** The session was not accepted; see UNAUTHORIZED_RETRIES. */
export const CLOSE_UNAUTHORIZED = 4401;

/**
 * A 401 is retried with a freshly minted token, since a token can expire between
 * being minted and being checked. One that is refused again, on a connection that
 * never got as far as subscribing, is final: the session is gone, and retrying would
 * loop for as long as the page stays open.
 */
const UNAUTHORIZED_RETRIES = 1;

const INITIAL_BACKOFF_MS = 1_000;
const MAX_BACKOFF_MS = 30_000;

export interface RealtimeOptions {
  streamId: number;
  /** Every event, in order, including the ones also passed to `onResync`. */
  onEvent?: (event: RealtimeEvent) => void;
  /**
   * Fetch state afresh. Called once a connection is subscribed — the first frame has
   * arrived, so nothing published from then on can be missed — and on a ResyncEvent.
   * That includes the first connection, so a snapshot loaded here cannot race the
   * socket.
   */
  onResync?: () => void;
  onStatus?: (status: RealtimeStatus) => void;
  /**
   * The server refused the stream for good; no reconnect follows. Code 4401 here means
   * the session has expired or been revoked: the user has to sign in again.
   */
  onFatal?: (code: number, reason: string) => void;
  /**
   * The server closed the socket, for any reason, before `onFatal` or a retry. Lets a
   * view say *why* it is reconnecting — a 4401 that keeps coming back means the
   * session has expired, which a retry alone never fixes.
   */
  onClose?: (code: number, reason: string) => void;

  /** A token for the first frame, or null for anonymous. Overridable in tests. */
  getToken?: () => Promise<string | null>;
  /** Overridable in tests; defaults to this origin. */
  url?: string;
  /** [0, 1), for the backoff's jitter. Overridable in tests. */
  random?: () => number;
}

/** Exponential backoff with equal jitter: half the delay fixed, half random. */
export function backoffDelay(attempt: number, random: () => number = Math.random): number {
  const ceiling = Math.min(MAX_BACKOFF_MS, INITIAL_BACKOFF_MS * 2 ** attempt);
  return ceiling / 2 + random() * (ceiling / 2);
}

function defaultUrl(streamId: number): string {
  const scheme = window.location.protocol === "https:" ? "wss" : "ws";
  return `${scheme}://${window.location.host}/api/v2/realtime?stream=${streamId}`;
}

export class RealtimeConnection {
  private socket: WebSocket | null = null;
  private retryTimer: ReturnType<typeof setTimeout> | null = null;
  private attempt = 0;
  /** Consecutive 4401s without a subscription in between. */
  private unauthorized = 0;
  private closed = false;
  private readonly listeners = new Map<
    RealtimeEventCase,
    Set<(value: never, event: RealtimeEvent) => void>
  >();

  constructor(private readonly options: RealtimeOptions) {}

  /** Opens the socket. Reconnects by itself until close() or a fatal refusal. */
  connect(): void {
    this.closed = false;
    void this.open();
  }

  /** Closes the socket for good. */
  close(): void {
    this.closed = true;
    if (this.retryTimer !== null) {
      clearTimeout(this.retryTimer);
      this.retryTimer = null;
    }
    const socket = this.socket;
    this.socket = null;
    socket?.close(1000);
    this.options.onStatus?.("closed");
  }

  /** Subscribes to one kind of event. Returns the unsubscribe. */
  on<C extends RealtimeEventCase>(
    kind: C,
    handler: (value: RealtimeEventValue<C>, event: RealtimeEvent) => void,
  ): () => void {
    let set = this.listeners.get(kind);
    if (!set) {
      set = new Set();
      this.listeners.set(kind, set);
    }
    const stored = handler as (value: never, event: RealtimeEvent) => void;
    set.add(stored);
    return () => set.delete(stored);
  }

  private async open(): Promise<void> {
    this.options.onStatus?.("connecting");

    let token: string | null;
    try {
      // Minted per connection: a reconnect may come long after the last token expired.
      token = await (this.options.getToken ?? freshTokenOrNull)();
    } catch {
      this.scheduleReconnect();
      return;
    }
    if (this.closed) return;

    const socket = new WebSocket(this.options.url ?? defaultUrl(this.options.streamId));
    this.socket = socket;
    let subscribed = false;

    socket.onopen = () => {
      socket.send(JSON.stringify(token ? { token } : {}));
    };

    socket.onmessage = (frame: MessageEvent) => {
      let event: RealtimeEvent;
      try {
        event = fromJson(RealtimeEventSchema, JSON.parse(String(frame.data)) as JsonValue, {
          ignoreUnknownFields: true,
        });
      } catch {
        return; // a frame this client cannot read is one it cannot act on either
      }

      this.dispatch(event);

      if (!subscribed) {
        subscribed = true;
        this.attempt = 0;
        this.unauthorized = 0;
        this.options.onStatus?.("open");
        this.options.onResync?.();
      } else if (event.event.case === "resync") {
        this.options.onResync?.();
      }
    };

    socket.onclose = (close: CloseEvent) => {
      if (this.socket !== socket) return; // replaced or closed on purpose
      this.socket = null;
      this.options.onClose?.(close.code, close.reason);

      if (close.code === CLOSE_UNAUTHORIZED && this.unauthorized++ >= UNAUTHORIZED_RETRIES) {
        this.giveUp(close);
        return;
      }

      if (FATAL_CLOSE_CODES.has(close.code)) {
        this.giveUp(close);
        return;
      }
      this.scheduleReconnect();
    };
  }

  private giveUp(close: CloseEvent | { code: number; reason: string }): void {
    this.closed = true;
    this.options.onStatus?.("closed");
    this.options.onFatal?.(close.code, close.reason);
  }

  private dispatch(event: RealtimeEvent): void {
    this.options.onEvent?.(event);
    const kind = event.event.case;
    if (!kind) return;
    for (const handler of this.listeners.get(kind) ?? []) {
      handler(event.event.value as never, event);
    }
  }

  private scheduleReconnect(): void {
    if (this.closed) return;
    this.options.onStatus?.("connecting");
    const delay = backoffDelay(this.attempt, this.options.random);
    this.attempt += 1;
    this.retryTimer = setTimeout(() => {
      this.retryTimer = null;
      void this.open();
    }, delay);
  }
}
