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
 * A refusal (4400/4403/4404) is final and says so. A 4401 is retried by the connection
 * — tokens expire — but when it keeps coming back the session is gone, and only the
 * viewer can fix that, so it is reported as an error even while reconnecting.
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

  if (s.lastCloseCode === 4401) {
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
