import { defineStore } from "pinia";
import { ref } from "vue";

import {
  fetchActivePoll,
  fetchChatMessages,
  sendMessage as postMessage,
  toggleReaction as postReaction,
  vote as postVote,
  toChatMessage,
  toChatPoll,
  toChatReaction,
  type ChatMessage,
  type ChatPoll,
  type NewChatMessage,
} from "@/lib/chat";
import {
  CLOSE_UNAUTHORIZED,
  RealtimeConnection,
  type RealtimeEvent,
  type RealtimeStatus,
} from "@/lib/realtime";

/**
 * One stream's chat, kept current by the realtime socket.
 *
 * The state is a snapshot plus the events since. The snapshot is (re)loaded whenever
 * the connection says it may have missed something — including once it first
 * subscribes, so the initial load cannot race the socket. Events that arrive while a
 * snapshot is loading are held and applied on top of it, which is safe because every
 * event either replaces a field or upserts by id.
 */
export function useChatStore(streamId: number) {
  return defineStore(`chat/${streamId}`, () => {
    const messages = ref<ChatMessage[]>([]);
    const activePoll = ref<ChatPoll | undefined>(undefined);
    /** The poll that closed last, with its results. */
    const closedPoll = ref<ChatPoll | undefined>(undefined);
    const viewers = ref(0);
    /** Pushed changes to the stream itself; undefined until one arrives. */
    const live = ref<boolean | undefined>(undefined);
    const title = ref<string | undefined>(undefined);
    const descriptionHtml = ref<string | undefined>(undefined);

    const status = ref<RealtimeStatus>("closed");
    const loaded = ref(false);
    const error = ref<string | undefined>(undefined);
    /** The close code of a refusal that will not be retried: 4400, 4403, 4404, or a 4401 refused twice in a row. */
    const fatalCode = ref<number | undefined>(undefined);
    /** The close code of the last time the server hung up, until a connection holds. */
    const lastCloseCode = ref<number | undefined>(undefined);
    /**
     * The socket was refused because the session ended. The chat as last loaded stays
     * readable, but nothing live arrives and nothing can be written until the user
     * signs in again.
     */
    const sessionExpired = ref(false);

    let connection: RealtimeConnection | null = null;
    /** Events received while a snapshot is in flight, applied once it lands. */
    let held: RealtimeEvent[] | null = null;
    /** Which snapshot is current, so an older one finishing late is discarded. */
    let generation = 0;

    function findMessage(id: number): ChatMessage | undefined {
      for (const m of messages.value) {
        if (m.id === id) return m;
        const reply = m.replies.find((r) => r.id === id);
        if (reply) return reply;
      }
      return undefined;
    }

    function upsertMessage(incoming: ChatMessage): void {
      const siblings =
        incoming.replyTo === undefined
          ? messages.value
          : messages.value.find((m) => m.id === incoming.replyTo)?.replies;
      if (!siblings) return; // a reply to a message this client cannot see

      const i = siblings.findIndex((m) => m.id === incoming.id);
      if (i < 0) {
        siblings.push(incoming);
        return;
      }
      // An approval re-sends the message; keep any replies it arrived without. The
      // event for an anonymous message leaves out its author, whom the response to
      // sending it named to the author alone: keep that too.
      const known = siblings[i]!;
      const replies = incoming.replies.length > 0 ? incoming.replies : known.replies;
      siblings[i] = { ...incoming, userId: incoming.userId ?? known.userId, replies };
    }

    function removeMessage(id: number): void {
      // Deleting a message deletes its replies with it, which this does by itself.
      const top = messages.value.findIndex((m) => m.id === id);
      if (top >= 0) {
        messages.value.splice(top, 1);
        return;
      }
      for (const m of messages.value) {
        const i = m.replies.findIndex((r) => r.id === id);
        if (i >= 0) {
          m.replies.splice(i, 1);
          return;
        }
      }
    }

    /** Applies one event to the state. Exposed for tests and for optimistic updates. */
    function apply(event: RealtimeEvent): void {
      if (held !== null) {
        held.push(event);
        return;
      }

      const e = event.event;
      switch (e.case) {
        case "message":
          if (e.value.message) upsertMessage(toChatMessage(e.value.message));
          break;
        case "messageDeleted":
          removeMessage(e.value.messageId);
          break;
        case "messageResolved": {
          const m = findMessage(e.value.messageId);
          if (m) m.resolved = true;
          break;
        }
        case "messageRetracted": {
          // Kept, hidden: an administrator still lists it, and the view decides who
          // sees a message that is not visible.
          const m = findMessage(e.value.messageId);
          if (m) {
            m.visible = false;
            m.reactions = [];
          }
          break;
        }
        case "reactions": {
          const m = findMessage(e.value.messageId);
          if (m) m.reactions = e.value.reactions.map(toChatReaction);
          break;
        }
        case "pollStarted":
          if (e.value.poll) activePoll.value = toChatPoll(e.value.poll);
          closedPoll.value = undefined;
          break;
        case "pollClosed":
          activePoll.value = undefined;
          closedPoll.value = e.value.poll ? toChatPoll(e.value.poll) : undefined;
          break;
        case "pollVote": {
          const option = activePoll.value?.options.find((o) => o.id === e.value.pollOptionId);
          if (option) option.votes = e.value.votes;
          break;
        }
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
        case "resync":
        case undefined:
          // The connection turns a resync into a snapshot; nothing to apply.
          break;
      }
    }

    /** Fetches the snapshot, holding the events that arrive meanwhile. */
    async function snapshot(): Promise<void> {
      const mine = ++generation;
      held ??= [];
      try {
        const [m, poll] = await Promise.all([
          fetchChatMessages(streamId),
          fetchActivePoll(streamId),
        ]);
        if (mine !== generation) return; // a newer snapshot owns the held events
        messages.value = m;
        activePoll.value = poll;
        loaded.value = true;
        // A snapshot that loads after the session ended must not hide the reason
        // nothing live is arriving.
        if (!sessionExpired.value) error.value = undefined;
      } catch (err) {
        if (mine !== generation) return;
        error.value = err instanceof Error ? err.message : String(err);
      }

      const replay = held ?? [];
      held = null;
      replay.forEach(apply);
    }

    /** Connects, and loads the chat once subscribed. Idempotent. */
    function start(): void {
      if (connection) return;
      connection = new RealtimeConnection({
        streamId,
        onEvent: apply,
        onResync: () => void snapshot(),
        onStatus: (s) => {
          status.value = s;
          if (s === "open") lastCloseCode.value = undefined;
        },
        onClose: (code) => (lastCloseCode.value = code),
        onFatal: (code, reason) => {
          fatalCode.value = code;
          sessionExpired.value = code === CLOSE_UNAUTHORIZED;
          error.value = sessionExpired.value
            ? "your session has expired"
            : reason || "the chat is not available";
          // Still worth showing what the chat holds; the read decides for itself.
          void snapshot();
        },
      });
      connection.connect();
    }

    /**
     * Writes a message. The socket pushes it back to every viewer, this one included;
     * adding the response now as well makes it appear without waiting, and the upsert
     * makes the second arrival a no-op. Rejects with ChatCooldownError on a 429.
     */
    async function sendMessage(input: NewChatMessage): Promise<ChatMessage> {
      const sent = await postMessage(streamId, input);
      upsertMessage(sent);
      return sent;
    }

    /** Reacts with an emoji, or takes the reaction back. */
    async function toggleReaction(messageId: number, emoji: string): Promise<void> {
      const reactions = await postReaction(streamId, messageId, emoji);
      const m = findMessage(messageId);
      if (m) m.reactions = reactions;
    }

    /** Votes in the running poll. */
    async function vote(pollOptionId: number): Promise<void> {
      const voted = await postVote(streamId, pollOptionId);
      // Only if it is still the running poll: it may have closed meanwhile.
      if (activePoll.value?.id === voted.id) activePoll.value = voted;
    }

    function stop(): void {
      connection?.close();
      connection = null;
    }

    return {
      messages,
      activePoll,
      closedPoll,
      viewers,
      live,
      title,
      descriptionHtml,
      status,
      loaded,
      error,
      fatalCode,
      lastCloseCode,
      sessionExpired,
      sendMessage,
      toggleReaction,
      vote,
      apply,
      snapshot,
      start,
      stop,
    };
  })();
}
