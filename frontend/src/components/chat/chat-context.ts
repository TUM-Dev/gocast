import type { InjectionKey } from "vue";

import type { ChatAccess, ChatMessage } from "@/lib/chat";

/**
 * What a message needs from the panel around it: who the viewer is, what they may do
 * and the actions themselves. Provided rather than passed down because messages nest
 * and every level wants all of it. Absent, a message is read-only.
 */
export interface ChatContext {
  access: ChatAccess;
  /** The signed-in viewer, for telling their own reactions apart. */
  userId?: number;
  /** Writing is off for now: the session ended. */
  writeBlocked: boolean;
  react(message: ChatMessage, emoji: string): void;
  reply(message: ChatMessage): void;
  approve(message: ChatMessage): void;
  retract(message: ChatMessage): void;
  remove(message: ChatMessage): void;
  resolve(message: ChatMessage): void;
}

export const chatContextKey: InjectionKey<() => ChatContext> = Symbol("chatContext");
