import { Emoji, TopEmojis } from "top-twitter-emojis-map";

export class EmojiPicker {
    id: string;
    isOpen: boolean;

    static suggestions: Emoji[] = ["👍", "👎", "😄", "🎉", "😕", "❤️", "👀"].map((e) =>
        TopEmojis.find(({ emoji }) => emoji === e),
    );

    static getEmojiIndex(emojiName: string): number {
        return this.suggestions.findIndex((e) => e.short_names[0] === emojiName);
    }

    constructor(id: string) {
        this.id = id;
        this.isOpen = false;
    }

    getSuggestions(): Emoji[] {
        return EmojiPicker.suggestions;
    }

    eventOwner(e: CustomEvent): boolean {
        return e.detail.id == this.id;
    }

    onSelect(emoji) {
        window.dispatchEvent(
            new CustomEvent("emojipickeronselect", { detail: { id: this.id, emoji: emoji.short_names[0] } }),
        );
    }

    open() {
        if (this.isOpen) {
            return;
        }
        window.dispatchEvent(new CustomEvent("emojipickeropen", { detail: { id: this.id } }));
        setTimeout(() => (this.isOpen = true), 10);
    }

    close() {
        if (!this.isOpen) {
            return;
        }
        window.dispatchEvent(new CustomEvent("emojipickerclose", { detail: { id: this.id } }));
        this.isOpen = false;
    }
}
