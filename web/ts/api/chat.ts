import { get } from "../utilities/fetch-wrappers";
import { EmojiPicker } from "../chat/EmojiPicker";
import { ChatMessagePreprocessor } from "../chat/ChatMessagePreprocessor";

export class ChatMessage {
    readonly ID: number;
    readonly admin: boolean;

    readonly userId: number;
    readonly message: string;
    readonly name: string;
    readonly color: string;

    replies: ChatMessage[];
    replyTo: { Int64: number; Valid: boolean };

    reactions: ChatReaction[];
    aggregatedReactions: ChatReactionGroup[];

    visible: boolean;

    getLikes(): number {
        const g = this.aggregatedReactions.find((r) => r.emojiName === EmojiPicker.LikeEmojiName);
        return g ? g.reactions.length : 0;
    }
}

export type ChatReaction = {
    userID: number;
    username: string;
    emoji: string;
};

export type ChatReactionGroup = {
    emoji: string;
    emojiName: string;
    names: string[];
    namesPretty: string;
    reactions: ChatReaction[];
    hasReacted: boolean;
};

export class ChatMessageArray {
    private messages: ChatMessage[];

    static EmptyArray(): ChatMessageArray {
        return new ChatMessageArray([]);
    }

    constructor(messages: ChatMessage[]) {
        this.messages = messages.map((m) => Object.assign(new ChatMessage(), m));
    }

    forEach(callback: (obj: ChatMessage, i: number) => void) {
        this.messages.forEach(callback);
    }

    get(sortFn?: (a: ChatMessage, b: ChatMessage) => number): ChatMessage[] {
        return sortFn ? [...this.messages].sort(sortFn) : this.messages;
    }

    setReaction(reaction: { reactions: number; payload: ChatReaction[] }, userId: number) {
        const msg = this.messages.find((m) => m.ID === reaction.reactions);
        if (msg != undefined) {
            msg.reactions = reaction.payload;
            ChatMessagePreprocessor.AggregateReactions(msg, userId);
        }
    }

    pushReply(m: ChatMessage) {
        const base = this.messages.find((msg) => msg.ID === m.replyTo.Int64);
        if (base !== undefined) {
            base.replies.push(Object.assign(new ChatMessage(), m));
        }
    }

    pushMessage(m: ChatMessage) {
        this.messages.push(Object.assign(new ChatMessage(), m));
    }
}

/**
 * REST API Wrapper for /api/chat
 */
export const ChatAPI = {
    async getMessages(streamId: number): Promise<ChatMessageArray> {
        return get(`/api/chat/${streamId}/messages`).then((messages: ChatMessage[]) => new ChatMessageArray(messages));
    },
};
