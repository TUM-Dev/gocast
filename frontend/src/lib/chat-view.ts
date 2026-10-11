/**
 * What the chat panel shows, decided apart from how it is drawn: which notice the
 * connection earns, how a reaction is spelled and how a poll's votes are shared out.
 */

import type { ChatMessage, ChatPoll } from "./chat";
import type { RealtimeStatus } from "./realtime";

/**
 * The reaction names the chat accepts, as the emoji they stand for. The server stores
 * the short name only, so an unknown one is shown as `:name:` rather than dropped.
 */
const REACTION_GLYPHS: Record<string, string> = {
  "+1": "\u{1F44D}",
  "-1": "\u{1F44E}",
  smile: "\u{1F604}",
  tada: "\u{1F389}",
  confused: "\u{1F615}",
  heart: "❤️",
  eyes: "\u{1F440}",
};

/** The reactions a message can get, in the order the picker offers them; the server's list. */
export const REACTIONS = ["+1", "-1", "smile", "tada", "confused", "heart", "eyes"] as const;

const REACTION_NAMES: Record<string, string> = {
  "+1": "Thumbs up",
  "-1": "Thumbs down",
  smile: "Smile",
  tada: "Celebrate",
  confused: "Confused",
  heart: "Heart",
  eyes: "Eyes",
};

/** What a screen reader says for a reaction. */
export function reactionName(name: string): string {
  return REACTION_NAMES[name] ?? name;
}

export function reactionGlyph(name: string): string {
  return REACTION_GLYPHS[name] ?? `:${name}:`;
}

export interface ConnectionNotice {
  /** `error` will not clear by itself; `info` is a connection on its way. */
  kind: "info" | "error";
  text: string;
  /** Offer the sign-in page next to the text. */
  signIn?: boolean;
}

export interface ConnectionState {
  status: RealtimeStatus;
  /** The first snapshot has landed. */
  loaded: boolean;
  error?: string;
  /** The close code of a refusal that will not be retried. */
  fatalCode?: number;
  /** The close code of the last hang-up, cleared once a connection holds. */
  lastCloseCode?: number;
}

/**
 * The line to show above the messages, or null when all is well.
 *
 * A refusal (4400/4403/4404) is final and says so. A 4401 is retried once by the
 * connection — tokens expire — and a second one in a row ends it as fatal: the session
 * is gone and only the viewer can fix that. The lastCloseCode check covers the retry
 * in between, so the notice does not flicker to "reconnecting" first.
 */
export function connectionNotice(s: ConnectionState): ConnectionNotice | null {
  switch (s.fatalCode) {
    case 4400:
      return { kind: "error", text: "This is not a valid chat address." };
    case 4403:
      return { kind: "error", text: "You are not allowed to join this chat.", signIn: true };
    case 4404:
      return { kind: "error", text: "This stream does not exist." };
  }

  if (s.fatalCode === 4401 || s.lastCloseCode === 4401) {
    return { kind: "error", text: "Your session has expired. Sign in to keep the chat live.", signIn: true };
  }

  if (s.status === "connecting") {
    return { kind: "info", text: s.loaded ? "Connection lost. Reconnecting…" : "Connecting…" };
  }

  if (s.error) {
    return { kind: "error", text: `The chat could not be loaded: ${s.error}` };
  }
  return null;
}

/** Every message in the list, replies included. */
export function countMessages(messages: ChatMessage[]): number {
  return messages.reduce((n, m) => n + 1 + m.replies.length, 0);
}

export function pollTotalVotes(poll: ChatPoll): number {
  return poll.options.reduce((sum, o) => sum + o.votes, 0);
}

/** An option's share of the votes in whole percent; 0 while nobody has voted. */
export function pollShare(poll: ChatPoll, votes: number): number {
  const total = pollTotalVotes(poll);
  return total === 0 ? 0 : Math.round((votes / total) * 100);
}

/** 14:05, in the viewer's locale and time zone. */
export function formatMessageTime(date: Date): string {
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

/**
 * Whether the viewer sees a message in the list. The server sends a held message to
 * the course's administrators and its author only, but a message retracted while the
 * viewer had it stays in their list until the page reloads, so the view enforces it too.
 */
export function isShown(message: ChatMessage, canModerate: boolean, userId?: number): boolean {
  return message.visible || canModerate || (userId !== undefined && message.userId === userId);
}

/** The messages the viewer sees, replies included, for display; the input is not changed. */
export function shownMessages(
  messages: ChatMessage[],
  canModerate: boolean,
  userId?: number,
): ChatMessage[] {
  return messages
    .filter((m) => isShown(m, canModerate, userId))
    .map((m) => ({ ...m, replies: m.replies.filter((r) => isShown(r, canModerate, userId)) }));
}

/** How many messages wait for an administrator, replies included. */
export function countAwaitingApproval(messages: ChatMessage[]): number {
  return messages.reduce(
    (n, m) => n + (m.visible ? 0 : 1) + m.replies.filter((r) => !r.visible).length,
    0,
  );
}

/** The longest message the server takes, in bytes once trimmed. */
export const MAX_MESSAGE_BYTES = 1000;

export function messageBytes(text: string): number {
  return new TextEncoder().encode(text.trim()).length;
}
