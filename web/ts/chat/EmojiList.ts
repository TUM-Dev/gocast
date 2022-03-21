import { Emoji, TopEmojis } from "top-twitter-emojis-map";
import { getCurrentWordPositions } from "./misc";

export class EmojiList {
    suggestions: Emoji[];
    emojiIndex: number;

    constructor() {
        this.suggestions = [];
        this.emojiIndex = 0;
    }

    valid(): boolean {
        return this.suggestions.length > 0;
    }
    onSuggestions(e) {
        this.suggestions = e.detail;
    }

    onSelectionTriggered(e) {
        if (this.valid()) {
            this.insertEmoji(this.suggestions[this.emojiIndex]);
        }
    }

    onArrowDown(e) {
        if (this.valid()) {
            this.emojiIndex = (this.emojiIndex + 1) % this.suggestions.length;
        }
    }

    onArrowUp(e) {
        if (this.valid()) {
            this.emojiIndex = (this.emojiIndex - 1) % this.suggestions.length;
        }
    }

    onInserted(e) {
        this.suggestions = [];
        this.emojiIndex = 0;
    }

    onChatEnter() {
        let event = "sendmessage";
        if (this.valid()) {
            event = "emojiselectiontriggered";
        }

        window.dispatchEvent(new CustomEvent(event));
    }

    getEmojisForMessage(message: string, cursorPos: number) {
        const pos = getCurrentWordPositions(message, cursorPos);
        const currentWord = message.substring(pos[0], pos[1]);
        if (!currentWord.startsWith(":") || currentWord.length < 2) {
            window.dispatchEvent(new CustomEvent("emojisuggestions", { detail: [] }));
            return;
        }

        const emojis = this.findEmojisForInput(currentWord.substring(1));
        window.dispatchEvent(new CustomEvent("emojisuggestions", { detail: emojis }));
    }

    private findEmojisForInput(input: string): Emoji[] {
        return TopEmojis.filter((emoji) => {
            return emoji.short_names.some((key) => key.startsWith(input));
        }).slice(0, 7);
    }

    private insertEmoji(emoji: Emoji) {
        const chatInput: HTMLInputElement = document.getElementById("chatInput") as HTMLInputElement;
        const pos = getCurrentWordPositions(chatInput.value, chatInput.selectionStart);
        // send new message to alpine
        window.dispatchEvent(
            new CustomEvent("setmessage", {
                detail: chatInput.value.substring(0, pos[0]) + emoji.emoji + " " + chatInput.value.substring(pos[1]),
            }),
        );
        chatInput.focus();
        chatInput.selectionStart = pos[0] + emoji.emoji.length + 1; // +1 for space
        chatInput.selectionEnd = pos[0] + emoji.emoji.length + 1;
        // notify alpine to remove emoji suggestions
        window.dispatchEvent(new CustomEvent("emojisinserted"));
    }
}
