import { ChatMessage } from "../api/chat";

export enum ChatSortMode {
    LiveChat,
    PopularFirst,
}

type CompareFn = (a: ChatMessage, b: ChatMessage) => number;

export abstract class ChatMessageSorter {
    static GetSortFn(sortMode: ChatSortMode): CompareFn {
        switch (sortMode) {
            case ChatSortMode.LiveChat:
                return (a: ChatMessage, b: ChatMessage) => (a.ID < b.ID ? -1 : 1);
            case ChatSortMode.PopularFirst:
                return (a: ChatMessage, b: ChatMessage) => {
                    const likesA = a.getLikes();
                    const likesB = b.getLikes();

                    if (likesA === likesB) {
                        return a.ID - b.ID; // same amount of likes -> newer messages up
                    }
                    return likesB - likesA; // more likes -> up
                };
        }
    }
}
