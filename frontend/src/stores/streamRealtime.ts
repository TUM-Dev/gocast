import { defineStore } from "pinia";
import { computed, ref } from "vue";

import {
  CLOSE_UNAUTHORIZED,
  RealtimeConnection,
  type RealtimeEvent,
  type RealtimeStatus,
} from "@/lib/realtime";

/** What a consumer of a stream's realtime socket wants to hear. All parts are optional. */
export interface RealtimeSubscriber {
  /** Every event, in order, after the store has applied its own. */
  onEvent?: (event: RealtimeEvent) => void;
  /**
   * Load state afresh: the socket is subscribed, or may have missed events. Called
   * straight away for a consumer that joins a socket which is subscribed already.
   */
  onResync?: () => void;
  /** The server refused the stream for good. Called straight away if it did so already. */
  onFatal?: (code: number, reason: string) => void;
}

/**
 * The realtime socket of one stream, shared by everything on the page that listens
 * to it: the chat today, the player's live state and the viewer count beside it
 * later. A tab holds one connection per stream however many consumers there are —
 * every extra socket is a viewer counted twice and a reconnect storm — so consumers
 * `subscribe()` rather than connect. The first subscription opens the socket and the
 * last unsubscribe closes it.
 *
 * The stream-level facts every consumer wants (viewers, live, title, description and
 * the state of the connection) are kept here; what is specific to a consumer, the
 * chat's messages for one, is its own business, fed by `onEvent`.
 */
export function useStreamRealtime(streamId: number) {
  return defineStore(`realtime/${streamId}`, () => {
    const viewers = ref(0);
    /** Pushed changes to the stream itself; undefined until one arrives. */
    const live = ref<boolean | undefined>(undefined);
    const title = ref<string | undefined>(undefined);
    const descriptionHtml = ref<string | undefined>(undefined);

    const status = ref<RealtimeStatus>("closed");
    /** The close code of a refusal that will not be retried: 4400, 4403, 4404, or a 4401 refused twice in a row. */
    const fatalCode = ref<number | undefined>(undefined);
    /** The close code of the last time the server hung up, until a connection holds. */
    const lastCloseCode = ref<number | undefined>(undefined);
    /**
     * The socket was refused because the session ended. What was loaded stays
     * readable, but nothing live arrives and nothing can be written until the user
     * signs in again.
     */
    const sessionExpired = computed(() => fatalCode.value === CLOSE_UNAUTHORIZED);

    // Not state: nothing renders from these.
    const subscribers = new Set<RealtimeSubscriber>();
    let connection: RealtimeConnection | null = null;
    /** The connection has subscribed, so a consumer that joins now has missed nothing since. */
    let synced = false;
    let fatal: { code: number; reason: string } | null = null;

    function each(call: (s: RealtimeSubscriber) => void): void {
      // A copy: a consumer may unsubscribe from inside its own callback.
      for (const s of [...subscribers]) call(s);
    }

    function applyOwn(event: RealtimeEvent): void {
      const e = event.event;
      switch (e.case) {
        case "viewers":
          viewers.value = e.value.viewers;
          break;
        case "live":
          live.value = e.value.live;
          break;
        case "title":
          title.value = e.value.title;
          break;
        case "description":
          descriptionHtml.value = e.value.html;
          break;
      }
    }

    function open(): void {
      synced = false;
      fatal = null;
      fatalCode.value = undefined;
      lastCloseCode.value = undefined;
      connection = new RealtimeConnection({
        streamId,
        onEvent: (event) => {
          applyOwn(event);
          each((s) => s.onEvent?.(event));
        },
        onResync: () => {
          synced = true;
          each((s) => s.onResync?.());
        },
        onStatus: (s) => {
          status.value = s;
          if (s === "open") lastCloseCode.value = undefined;
        },
        onClose: (code) => (lastCloseCode.value = code),
        onFatal: (code, reason) => {
          fatal = { code, reason };
          fatalCode.value = code;
          each((s) => s.onFatal?.(code, reason));
        },
      });
      connection.connect();
    }

    function close(): void {
      connection?.close();
      connection = null;
      synced = false;
      fatal = null;
      status.value = "closed";
      viewers.value = 0;
      live.value = title.value = descriptionHtml.value = undefined;
    }

    /**
     * Listens to the stream. Opens the socket if this is the first listener; the
     * returned function stops listening, and closes the socket if it was the last.
     */
    function subscribe(subscriber: RealtimeSubscriber): () => void {
      subscribers.add(subscriber);
      if (!connection) {
        open();
      } else {
        if (synced) subscriber.onResync?.();
        if (fatal) subscriber.onFatal?.(fatal.code, fatal.reason);
      }

      let active = true;
      return () => {
        if (!active) return;
        active = false;
        subscribers.delete(subscriber);
        if (subscribers.size === 0) close();
      };
    }

    return {
      viewers,
      live,
      title,
      descriptionHtml,
      status,
      fatalCode,
      lastCloseCode,
      sessionExpired,
      subscribe,
    };
  })();
}
