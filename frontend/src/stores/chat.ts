import { defineStore } from "pinia";
import { computed, ref } from "vue";

import {
  fetchActivePoll,
  fetchChatMessages,
  sendMessage as postMessage,
  toggleReaction as postReaction,
  vote as postVote,
  toChatMessage,
  toChatPoll,
  toChatReaction,
  NO_ACCESS,
  approveMessage,
  closePoll,
  deleteMessage,
  resolveMessage,
  retractMessage,
  startPoll,
  type ChatAccess,
  type ChatMessage,
  type ChatPoll,
  type NewChatMessage,
} from "@/lib/chat";
import { CLOSE_UNAUTHORIZED, type RealtimeEvent } from "@/lib/realtime";
import { useStreamRealtime } from "@/stores/streamRealtime";

/**
 * One stream's chat, kept current by the stream's shared realtime socket.
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
    /** What the caller may do; read-only until the first snapshot says otherwise. */
    const access = ref<ChatAccess>(NO_ACCESS);

    // The socket is the stream's, shared with everything else on the page listening to
    // it; the connection's state is read through from there.
    const realtime = useStreamRealtime(streamId);
    const viewers = computed(() => realtime.viewers);
    const status = computed(() => realtime.status);
    const fatalCode = computed(() => realtime.fatalCode);
    const lastCloseCode = computed(() => realtime.lastCloseCode);
    const sessionExpired = computed(() => realtime.sessionExpired);

    const loaded = ref(false);
    const error = ref<string | undefined>(undefined);

    let unsubscribe: (() => void) | null = null;
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

    /**
     * Kept, hidden: an administrator still lists a retracted message, and the view
     * decides who sees a message that is not visible.
     */
    function hideMessage(id: number): void {
      const m = findMessage(id);
      if (m) {
        m.visible = false;
        m.retracted = true;
        m.reactions = [];
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
        case "messageRetracted":
          hideMessage(e.value.messageId);
          break;
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
        default:
          // The stream's own events are the shared socket's; a resync reaches us as a snapshot.
          break;
      }
    }

    /** Fetches the snapshot, holding the events that arrive meanwhile. */
    async function snapshot(): Promise<void> {
      const mine = ++generation;
      held ??= [];
      try {
        const [chat, poll] = await Promise.all([
          fetchChatMessages(streamId),
          fetchActivePoll(streamId),
        ]);
        if (mine !== generation) return; // a newer snapshot owns the held events
        messages.value = chat.messages;
        access.value = chat.access;
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

    /** Listens to the socket, and loads the chat once it has subscribed. Idempotent. */
    function start(): void {
      if (unsubscribe) return;
      unsubscribe = realtime.subscribe({
        onEvent: apply,
        onResync: () => void snapshot(),
        onFatal: (code, reason) => {
          error.value = code === CLOSE_UNAUTHORIZED ? "your session has expired" : reason || "the chat is not available";
          // Still worth showing what the chat holds; the read decides for itself.
          void snapshot();
        },
      });
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

    // ----- Moderation and polls, for the course's administrators -----
    // Each call applies its own result as well as waiting for the socket's event, as
    // sendMessage does: the events are idempotent, so the second arrival is a no-op.

    /** Publishes a message that awaited approval to every viewer. */
    async function approve(messageId: number): Promise<void> {
      upsertMessage(await approveMessage(streamId, messageId));
    }

    /** Hides an approved message again; it keeps its place for administrators and its author. */
    async function retract(messageId: number): Promise<void> {
      await retractMessage(streamId, messageId);
      hideMessage(messageId);
    }

    /** Deletes a message, and its replies, for everyone. */
    async function remove(messageId: number): Promise<void> {
      await deleteMessage(streamId, messageId);
      removeMessage(messageId);
    }

    /** Marks a message as resolved. */
    async function resolve(messageId: number): Promise<void> {
      await resolveMessage(streamId, messageId);
      const m = findMessage(messageId);
      if (m) m.resolved = true;
    }

    /** Starts a poll. Rejects with a 409 ApiError while another one is running. */
    async function openPoll(question: string, answers: string[]): Promise<void> {
      activePoll.value = await startPoll(streamId, question, answers);
      closedPoll.value = undefined;
    }

    /** Closes the running poll and keeps its results for display. */
    async function endPoll(): Promise<void> {
      const results = await closePoll(streamId);
      activePoll.value = undefined;
      closedPoll.value = results;
    }

    /** Stops showing the results of the poll that closed. */
    function dismissClosedPoll(): void {
      closedPoll.value = undefined;
    }

    function stop(): void {
      unsubscribe?.();
      unsubscribe = null;
    }

    return {
      messages,
      activePoll,
      closedPoll,
      access,
      viewers,
      status,
      loaded,
      error,
      fatalCode,
      lastCloseCode,
      sessionExpired,
      sendMessage,
      toggleReaction,
      vote,
      approve,
      retract,
      remove,
      resolve,
      openPoll,
      endPoll,
      dismissClosedPoll,
      apply,
      snapshot,
      start,
      stop,
    };
  })();
}
