import { get } from "../utilities/fetch-wrappers";
import { ChatMessagePreprocessor } from "../chat/ChatMessagePreprocessor";
import { User } from "./users";
import { ToggleableElement } from "../utilities/ToggleableElement";

export class ChatMessage implements Identifiable {
    readonly ID: number;
    readonly admin: boolean;

    message: string;
    readonly userId: number;
    readonly name: string;
    readonly color: string;

    replies: ChatMessage[];
    replyTo: { Int64: number; Valid: boolean };

    reactions: ChatReaction[];
    aggregatedReactions: ChatReactionGroup[];

    addressedTo: number[];
    visible: boolean;
    resolved: boolean;

    isGrayedOut: boolean;

    ShowReplies = new ToggleableElement();
    ShowEmojiPicker = new ToggleableElement();
    ShowAdminTools = new ToggleableElement();

    CreatedAt: string;

    getLikes(): number {
        const g = this.aggregatedReactions.find((r) => r.emojiName === "+1");
        return g ? g.reactions.length : 0;
    }

    friendlyCreatedAt(): string {
        const d = new Date(this.CreatedAt);
        return ("0" + d.getHours()).slice(-2) + ":" + ("0" + d.getMinutes()).slice(-2);
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
        this.messages.forEach((msg) => {
            msg.replies = msg.replies.map((reply) => Object.assign(new ChatMessage(), reply));
        });
    }

    forEach(callback: (obj: ChatMessage, i: number) => void) {
        this.messages.forEach(callback);
    }

    get(sortFn?: (a: ChatMessage, b: ChatMessage) => number): ChatMessage[] {
        return sortFn ? [...this.messages].sort(sortFn) : this.messages;
    }

    resolve(msg: Identifiable) {
        this.messages.find((m) => m.ID === msg.ID).resolved = true;
    }

    delete(msg: Identifiable) {
        this.messages = this.messages.filter((m) => m.ID != msg.ID);
    }

    approve(msg: ChatMessage) {
        const filtered = this.messages.filter((m) => m.ID !== msg.ID);
        filtered.push(Object.assign(new ChatMessage(), msg));
        this.messages = filtered;
    }

    retract(msg: ChatMessage, isAdmin: boolean) {
        if (isAdmin) {
            this.messages.find((m) => m.ID === msg.ID).visible = false;
        } else {
            this.messages = this.messages.filter((m) => m.ID !== msg.ID);
        }
    }

    setReaction(reaction: { reactions: number; payload: ChatReaction[] }, user: User) {
        const msg = this.messages.find((m) => m.ID === reaction.reactions);
        if (msg != undefined) {
            msg.reactions = reaction.payload;
            ChatMessagePreprocessor.AggregateReactions(msg, user);
        }
    }

    pushReply(m: ChatMessage) {
        const base = this.messages.find((msg) => msg.ID === m.replyTo.Int64);
        if (base !== undefined) {
            if (base.replies.findIndex((msg) => msg.ID === m.ID) === -1) {
                base.replies.push(Object.assign(new ChatMessage(), m));
            }
        }
    }

    pushMessage(m: ChatMessage) {
        if (this.messages.findIndex((msg) => msg.ID === m.ID) === -1) {
            this.messages.push(Object.assign(new ChatMessage(), m));
        }
    }
}

export class Poll implements Identifiable {
    ID: number;
    options: PollOption[];
    question: string;

    submitted: boolean;

    getOptionWidth(pollOption) {
        const minWidth = 1;
        const maxWidth = 100;
        const maxVotes = Math.max(...this.options.map(({ votes: v }) => v));

        if (pollOption.votes == 0) return `${minWidth.toString()}%`;

        const fractionOfMax = pollOption.votes / maxVotes;
        const fractionWidth = minWidth + fractionOfMax * (maxWidth - minWidth);
        return `${Math.ceil(fractionWidth).toString()}%`;
    }
}

export class PollOption implements Identifiable {
    ID: number;
    answer: string;
    votes: number;
}

export type ChatUser = {
    id: number;
    name: string;
};

/**
 * REST API Wrapper for /api/chat
 */
export const ChatAPI = {
    async getMessages(streamId: number): Promise<ChatMessageArray> {
        return get(`/api/chat/${streamId}/messages`).then((messages: ChatMessage[]) => new ChatMessageArray(messages));
    },

    async getUsers(streamId: number): Promise<ChatUser[]> {
        return get(`/api/chat/${streamId}/users`);
    },

    async getPolls(streamId: number): Promise<Poll[]> {
        return get(`/api/chat/${streamId}/polls`).then((polls) => polls.map((poll) => Object.assign(new Poll(), poll)));
    },

    async getActivePoll(streamId: number): Promise<Poll> {
        return get(`/api/chat/${streamId}/active-poll`, {}, true)
            .then((poll) => Object.assign(new Poll(), poll))
            .catch((err) => ({ ID: -1 }));
    },
};
