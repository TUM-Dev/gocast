import { AlpineComponent } from "./alpine-component";
import { Emoji, TopEmojis } from "top-twitter-emojis-map";
import { getCurrentWordPositions } from "../chat/misc";
import { SocketConnections, NewChatMessage } from "../api/chat-ws";
import { ChatMessage } from "../api/chat";

export function chatPromptContext(): AlpineComponent {
    return {
        message: "" as string,
        isAnonymous: false as boolean,
        addressedTo: [] as ChatUser[],
        reply: NewReply,

        emojis: new EmojiSuggestions(),

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
            SocketConnections.chat.sendMessage({
                msg: this.message,
                anonymous: this.isAnonymous,
                replyTo: this.reply.id,
                addressedTo: this.addressedTo.map((u) => u.id),
            });
            this.reset();
        },

        /*
            Remove last occurrence of emoji short_name, e.g. "i like my :dog" => "i like my "
            and add emoji to message
         */
        addEmoji(emoji: string) {
            // this.message = this.message.replace(/:\w*$/g, "");
            const pos = getCurrentWordPositions(this.input.value, this.input.selectionStart);
            this.message = this.message.substring(0, pos[0]) + emoji + " " + this.message.substring(pos[1]);
            this.emojis.reset();
        },

        keyup(e) {
            console.log("🌑 keyup '", this.message, "'");
            this.addressedTo = this.addressedTo.filter((user) => this.message.includes(`@${user.name}`));
            this.emojis.getSuggestionsForMessage(e.target.value, e.target.selectionStart);
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
