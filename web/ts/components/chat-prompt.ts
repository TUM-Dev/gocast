import { AlpineComponent } from "./alpine-component";
import { Emoji, TopEmojis } from "top-twitter-emojis-map";
import { getCurrentWordPositions } from "../chat/misc";
import { SocketConnections, ChatWebsocketConnection } from "../api/chat-ws";
import { ChatAPI, ChatMessage } from "../api/chat";
import { SetReply, Tunnel } from "../utilities/tunnels";
import { isAlphaNumeric, isSpacebar } from "../utilities/keycodes";
import { ToggleableElement } from "../utilities/ToggleableElement";

export function chatPromptContext(streamId: number): AlpineComponent {
    return {
        message: "" as string,
        isAnonymous: false as boolean,
        addressedTo: [] as ChatUser[],
        reply: NewReply.NoReply,

        emojis: new EmojiSuggestions(),
        users: new UserSuggestions(streamId),

        showOptions: new ToggleableElement(),

        ws: new ChatWebsocketConnection(SocketConnections.ws),

        inputEl: document.getElementById("chat-input") as HTMLTextAreaElement,
        usersEl: document.getElementById("chat-user-list") as HTMLInputElement,
        emojiEl: document.getElementById("chat-emoji-list") as HTMLInputElement,

        init() {
            console.log("🌑 init chat prompt");
            const callback = (sr: SetReply) => this.setReply(sr);
            Tunnel.reply.subscribe(callback);

            this.inputEl.focus();
        },

        reset() {
            this.message = "";
            this.addressedTo = [];
            this.reply = NewReply.NoReply;
            this.emojis.reset();
            this.users.reset();
        },

        send(event: KeyboardEvent) {
            if (event.shiftKey) {
                return;
            }
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
            const pos = getCurrentWordPositions(this.inputEl.value, this.inputEl.selectionStart);
            this.message = this.message.substring(0, pos[0]) + emoji + " " + this.message.substring(pos[1]);
            this.emojis.reset();
            this.inputEl.focus();
        },

        addAddressee(user: ChatUser): void {
            const pos = getCurrentWordPositions(this.inputEl.value, this.inputEl.selectionStart);

            // replace message with username e.g. 'Hello @Ad' to 'Hello @Admin':
            this.message =
                this.message.substring(0, pos[0]) +
                this.message.substring(pos[0], pos[1]).replace(/@(\w)*/, "@" + user.name) +
                " " +
                this.message.substring(pos[1] + this.message.substring(pos[0], pos[1]).length);

            this.addressedTo.push(user);
            this.users.reset();
            this.inputEl.focus();
        },

        keyup() {
            this.addressedTo = this.addressedTo.filter((user) => this.message.includes(`@${user.name}`));
            this.emojis.getSuggestionsForMessage(this.inputEl.value, this.inputEl.selectionStart);
            this.users.getSuggestionsForMessage(this.inputEl.value, this.inputEl.selectionStart);

            if (this.users.hasSuggestions()) {
                this.usersEl.focus();
            } else if (this.emojis.hasSuggestions()) {
                this.emojiEl.focus();
            }

            // https://stackoverflow.com/a/21079335
            this.inputEl.style.height = "0px";
            this.inputEl.style.height = this.inputEl.scrollHeight + "px";
        },

        keypressAlphanumeric(e) {
            if (isAlphaNumeric(e.keyCode) || isSpacebar(e.keyCode)) {
                this.inputEl.focus();
            }
        },

        backspace() {
            this.message = this.message.substring(0, this.message.length - 1);
            this.inputEl.focus();

            this.addressedTo = this.addressedTo.filter((user) => this.message.includes(`@${user.name}`));
            this.emojis.getSuggestionsForMessage(this.message, this.inputEl.selectionStart);
            this.users.getSuggestionsForMessage(this.message, this.inputEl.selectionStart);
        },

        setReply(sr: SetReply) {
            this.reset();
            this.reply = new NewReply(sr.message);
            this.inputEl.focus();
        },

        cancelReply() {
            this.reply = NewReply.NoReply;
        },

        openPopOut() {
            const height = window.innerHeight * 0.8;
            window.open(
                `${window.location.href}/chat/popup`,
                "tumlive-popout",
                `popup=yes,width=420,innerWidth=420,height=${height},innerHeight=${height}`,
            );
        },
    } as AlpineComponent;
}

export class EmojiSuggestions {
    suggestions: Emoji[];
    index: number;

    constructor() {
        this.suggestions = [];
        this.index = 0;
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

    selected(): Emoji {
        return this.suggestions[this.index];
    }

    next() {
        this.index = (this.index + 1) % this.suggestions.length;
        console.log(this.index);
    }

    prev() {
        this.index = (this.index - 1) % this.suggestions.length;
    }

    reset() {
        this.suggestions = [];
        this.index = 0;
    }
}

export class UserSuggestions {
    private all: ChatUser[];
    private readonly streamId: number;

    index: number;

    suggestions: ChatUser[];

    constructor(streamId: number) {
        this.all = this.suggestions = [];
        this.streamId = streamId;
        this.index = 0;
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

    selected(): ChatUser {
        return this.suggestions[this.index];
    }

    next() {
        this.index = (this.index + 1) % this.suggestions.length;
    }

    prev() {
        this.index = (this.index - 1) % this.suggestions.length;
    }

    reset() {
        this.suggestions = [];
        this.index = 0;
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
