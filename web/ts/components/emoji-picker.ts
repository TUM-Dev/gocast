import { AlpineComponent } from "./alpine-component";
import { TopEmojis } from "top-twitter-emojis-map";

export function emojiPickerContext(id: number): AlpineComponent {
    return {
        id: id,

        emojiSuggestions: ["👍", "👎", "😄", "🎉", "😕", "❤️", "👀"].map((e) =>
            TopEmojis.find(({ emoji }) => emoji === e),
        ),

        init() {},
    } as AlpineComponent;
}
