import { Emoji, TopEmojis } from "top-twitter-emojis-map";

export class EmojiPicker {
    suggestions: Emoji[];
    id: string;
    isOpen: boolean;

    constructor(id: string) {
        this.suggestions = ["👍", "👎", "😄", "🎉", "😕", "❤️", "👀"].map((e) =>
            TopEmojis.find(({ emoji }) => emoji === e),
        );
        this.id = id;
        this.isOpen = false;
    }

    eventOwner(e: CustomEvent): boolean {
        return e.detail.id == this.id;
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
