/**
 * A stream's chat: its messages, polls and the people writing in it.
 *
 * The generated types stop at this boundary, as in streams.ts: timestamps become
 * Dates and the zeros the wire uses for "none" become undefined.
 */

import { create } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";

import {
  GetActiveChatPollResponseSchema,
  ListChatMessagesResponseSchema,
  ListChatPollsResponseSchema,
  ListChatUsersResponseSchema,
  SendChatMessageRequestSchema,
  SendChatMessageResponseSchema,
  ToggleChatReactionRequestSchema,
  ToggleChatReactionResponseSchema,
  VoteChatPollRequestSchema,
  VoteChatPollResponseSchema,
  type ChatMessage as ChatMessageProto,
  type ChatPoll as ChatPollProto,
  type ChatReactionSummary,
} from "@/gen/server/apiv2_pb";
import { ApiError, apiGetMessage, apiGetMessageOptionalAuth, apiPostMessage } from "./api";

export interface ChatReaction {
  /** The emoji's short name, e.g. `+1` or `tada`. */
  emoji: string;
  count: number;
  userIds: number[];
  userNames: string[];
}

export interface ChatMessage {
  id: number;
  /** Undefined on an anonymous message, unless the caller wrote it. */
  userId?: number;
  userName: string;
  color: string;
  /** Sanitised by the server, links included. The only form to render as HTML. */
  html: string;
  /** As written, for quoting and editing. Never render it as HTML. */
  text: string;
  /** Written by one of the course's administrators. */
  admin: boolean;
  /** False while awaiting approval, or once retracted. */
  visible: boolean;
  resolved: boolean;
  /** The message this one replies to. */
  replyTo?: number;
  addressedTo: number[];
  createdAt: Date;
  reactions: ChatReaction[];
  replies: ChatMessage[];
}

export interface ChatPollOption {
  id: number;
  answer: string;
  /** 0 while the caller may not see the count. */
  votes: number;
}

export interface ChatPoll {
  id: number;
  question: string;
  active: boolean;
  options: ChatPollOption[];
  /** The option the caller voted for. */
  votedOptionId?: number;
}

export interface ChatUser {
  id: number;
  name: string;
}

export function toChatReaction(r: ChatReactionSummary): ChatReaction {
  return { emoji: r.emoji, count: r.count, userIds: [...r.userIds], userNames: [...r.userNames] };
}

export function toChatMessage(m: ChatMessageProto): ChatMessage {
  return {
    id: m.id,
    userId: m.userId || undefined,
    userName: m.userName,
    color: m.color,
    html: m.messageHtml,
    text: m.messageText,
    admin: m.admin,
    visible: m.visible,
    resolved: m.resolved,
    replyTo: m.replyTo || undefined,
    addressedTo: [...m.addressedTo],
    createdAt: m.createdAt ? timestampDate(m.createdAt) : new Date(0),
    reactions: m.reactions.map(toChatReaction),
    replies: m.replies.map(toChatMessage),
  };
}

export function toChatPoll(p: ChatPollProto): ChatPoll {
  return {
    id: p.id,
    question: p.question,
    active: p.active,
    options: p.options.map((o) => ({ id: o.id, answer: o.answer, votes: o.votes })),
    votedOptionId: p.votedOptionId || undefined,
  };
}

const base = (streamId: number) => `/streams/${streamId}/chat`;

/** The messages the caller may see, oldest first, replies nested. */
export async function fetchChatMessages(streamId: number): Promise<ChatMessage[]> {
  const res = await apiGetMessageOptionalAuth(
    ListChatMessagesResponseSchema,
    `${base(streamId)}/messages`,
  );
  return res.messages.map(toChatMessage);
}

/** The running poll, or undefined when there is none. */
export async function fetchActivePoll(streamId: number): Promise<ChatPoll | undefined> {
  const res = await apiGetMessageOptionalAuth(
    GetActiveChatPollResponseSchema,
    `${base(streamId)}/polls/active`,
  );
  return res.poll ? toChatPoll(res.poll) : undefined;
}

/** The closed polls with their results, newest first. Course administrators only. */
export async function fetchPolls(streamId: number): Promise<ChatPoll[]> {
  const res = await apiGetMessage(ListChatPollsResponseSchema, `${base(streamId)}/polls`);
  return res.polls.map(toChatPoll);
}

/** Who has written in the chat under their own name, for @-mentions. */
export async function fetchChatUsers(streamId: number): Promise<ChatUser[]> {
  const res = await apiGetMessageOptionalAuth(
    ListChatUsersResponseSchema,
    `${base(streamId)}/users`,
  );
  return res.users.map((u) => ({ id: u.id, name: u.name }));
}

/**
 * The server is holding the caller's messages back: more than five in two minutes.
 * Typed so a view can say "wait a moment" instead of showing a failure. Course
 * administrators are never held back.
 */
export class ChatCooldownError extends Error {
  constructor(message = "You are sending messages too fast. Please wait a bit.") {
    super(message);
    this.name = "ChatCooldownError";
  }
}

/** Runs a chat write, turning the cooldown's 429 into a ChatCooldownError. */
async function write<T>(call: () => Promise<T>): Promise<T> {
  try {
    return await call();
  } catch (err) {
    if (err instanceof ApiError && err.status === 429) {
      throw new ChatCooldownError();
    }
    throw err;
  }
}

export interface NewChatMessage {
  /** At most 1000 bytes once trimmed. */
  text: string;
  /** A top-level message of the same stream. */
  replyTo?: number;
  /** Hide the author's name; the server refuses it where the course disallows it. */
  anonymous?: boolean;
  /** Users to @-mention. */
  addressedTo?: number[];
}

/**
 * Writes a message. Resolves with it as stored, which the socket also pushes to every
 * viewer: callers that add the result to their list must dedupe by id.
 * Throws ChatCooldownError while the caller is cooling down.
 */
export async function sendMessage(streamId: number, input: NewChatMessage): Promise<ChatMessage> {
  const res = await write(() =>
    apiPostMessage(
      SendChatMessageRequestSchema,
      SendChatMessageResponseSchema,
      `${base(streamId)}/messages`,
      create(SendChatMessageRequestSchema, {
        streamId,
        text: input.text,
        replyTo: input.replyTo,
        anonymous: input.anonymous ?? false,
        addressedTo: input.addressedTo ?? [],
      }),
    ),
  );
  if (!res.message) throw new Error("the server answered without the message");
  return toChatMessage(res.message);
}

/** Reacts to a message with an emoji, or takes the reaction back. Resolves with the message's reactions. */
export async function toggleReaction(
  streamId: number,
  messageId: number,
  emoji: string,
): Promise<ChatReaction[]> {
  const res = await write(() =>
    apiPostMessage(
      ToggleChatReactionRequestSchema,
      ToggleChatReactionResponseSchema,
      `${base(streamId)}/messages/${messageId}/reactions`,
      create(ToggleChatReactionRequestSchema, { streamId, messageId, emoji }),
    ),
  );
  return res.reactions.map(toChatReaction);
}

/** Votes in the running poll; final. Resolves with the poll as the caller sees it. */
export async function vote(streamId: number, pollOptionId: number): Promise<ChatPoll> {
  const res = await write(() =>
    apiPostMessage(
      VoteChatPollRequestSchema,
      VoteChatPollResponseSchema,
      `${base(streamId)}/polls/votes`,
      create(VoteChatPollRequestSchema, { streamId, pollOptionId }),
    ),
  );
  if (!res.poll) throw new Error("the server answered without the poll");
  return toChatPoll(res.poll);
}
