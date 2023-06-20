import { AlpineComponent } from "./alpine-component";
import { Emoji, TopEmojis } from "top-twitter-emojis-map";
import { getCurrentWordPositions } from "../chat/misc";
import { SocketConnections, ChatWebsocketConnection } from "../api/chat-ws";
import { ChatAPI, ChatMessage } from "../api/chat";

export function chatPromptContext(streamId: number): AlpineComponent {
    return {
        message: "" as string,
        isAnonymous: false as boolean,
        addressedTo: [] as ChatUser[],
        reply: NewReply,

        emojis: new EmojiSuggestions(),
        users: new UserSuggestions(streamId),

        ws: new ChatWebsocketConnection(SocketConnections.ws),

        input: undefined as HTMLInputElement,

        init() {
            console.log("🌑 init chat prompt");
            this.input = document.getElementById("chat-input");
        },

        reset() {
            this.message = "";
            this.addressedTo = [];
            this.reply = NewReply.NoReply;
        },

        send() {
            console.log("🌑 send message '", this.message, "'");
            this.ws.sendMessage({
                msg: this.message,
                anonymous: this.isAnonymous,
                replyTo: this.reply.id,
                addressedTo: this.addressedTo.map((u) => u.id),
            });
            this.reset();
        },

        addEmoji(emoji: string) {
            const pos = getCurrentWordPositions(this.input.value, this.input.selectionStart);
            this.message = this.message.substring(0, pos[0]) + emoji + " " + this.message.substring(pos[1]);
            this.emojis.reset();
        },

        addAddressee(user: ChatUser): void {
            const pos = getCurrentWordPositions(this.input.value, this.input.selectionStart);

            // replace message with username e.g. 'Hello @Ad' to 'Hello @Admin':
            this.message =
                this.message.substring(0, pos[0]) +
                this.message.substring(pos[0], pos[1]).replace(/@(\w)*/, "@" + user.name) +
                " " +
                this.message.substring(pos[1] + this.message.substring(pos[0], pos[1]).length);

            this.addressedTo.push(user);
            this.users.reset();
        },

        keyup(e) {
            console.log("🌑 keyup '", this.message, "'");
            this.addressedTo = this.addressedTo.filter((user) => this.message.includes(`@${user.name}`));
            this.emojis.getSuggestionsForMessage(e.target.value, e.target.selectionStart);
            this.users.getSuggestionsForMessage(e.target.value, e.target.selectionStart);
        },
    } as AlpineComponent;
}

export class EmojiSuggestions {
    suggestions: Emoji[];
    emojiIndex: number;

    constructor() {
        this.suggestions = [];
        this.emojiIndex = 0;
    }

    reset() {
        this.suggestions = [];
    }

    hasSuggestions(): boolean {
        return this.suggestions.length > 0;
    }

    getSuggestionsForMessage(message: string, cursorPos: number) {
        const limit = 7;
        const pos = getCurrentWordPositions(message, cursorPos);
        const currentWord = message.substring(pos[0], pos[1]);

        if (!currentWord.startsWith(":") || currentWord.length < 2) {
            this.suggestions = [];
        } else {
            this.suggestions = TopEmojis.filter((emoji) => {
                return emoji.short_names.some((key) => key.startsWith(currentWord.substring(1)));
            }).slice(0, limit);
        }
    }
}

export class UserSuggestions {
    private all: ChatUser[];
    private readonly streamId: number;

    suggestions: ChatUser[];

    constructor(streamId: number) {
        this.all = this.suggestions = [];
        this.streamId = streamId;
    }

    hasSuggestions(): boolean {
        return this.suggestions.length > 0;
    }

    getSuggestionsForMessage(message: string, cursorPos: number) {
        const pos = getCurrentWordPositions(message, cursorPos);
        // substring(0,0) returns ''
        if (pos[0] === 0 && pos[1] === 0) pos[1] = 1;

        const currentWord = message.substring(pos[0], pos[1]);
        if (message === "" || !currentWord.startsWith("@")) {
            this.suggestions = [];
        } else if (currentWord === "@") {
            // load users on '@'
            ChatAPI.getUsers(this.streamId).then((users) => {
                this.all = this.suggestions = users;
            });
        } else {
            this.suggestions = this.all.filter((user) => user.name.startsWith(currentWord.substring(1)));
        }
    }

    reset() {
        this.suggestions = [];
    }
}

type ChatUser = {
    id: number;
    name: string;
};

class NewReply {
    message: ChatMessage;
    id: number;

    static NoReply = new NewReply({ ID: 0 } as ChatMessage);

    constructor(message: ChatMessage) {
        this.message = message;
        this.id = message.ID;
    }

    isNoReply(): boolean {
        return this.id === 0;
    }
}
