import { AlpineComponent } from "./alpine-component";
import { ChatAPI, ChatMessageArray } from "../api/chat";
import { WebsocketConnection } from "../chat/ws";
import { ChatMessageSorter, ChatSortMode } from "../chat/ChatMessageSorter";
import { ChatMessagePreprocessor } from "../chat/ChatMessagePreprocessor";
import { scrollChat, shouldScroll, showNewMessageIndicator } from "../chat";

export function chatContext(streamId: number): AlpineComponent {
    return {
        streamId: streamId as number,

        chatSortMode: ChatSortMode.LiveChat,
        chatSortFn: ChatMessageSorter.GetSortFn(ChatSortMode.LiveChat),
        messages: ChatMessageArray.EmptyArray() as ChatMessageArray,

        preprocessors: [ChatMessagePreprocessor.AggregateReactions],

        ws: new WebsocketConnection(`chat/${streamId}`),

        async init() {
            Promise.all([this.loadMessages(), this.initWebsocket()]).then(() => {
                this.messages.forEach((msg, _) => this.preprocessors.forEach((f) => f(msg, 1)));
            });
        },

        sortLiveFirst() {
            this.chatSortMode = ChatSortMode.LiveChat;
            this.chatSortFn = ChatMessageSorter.GetSortFn(ChatSortMode.LiveChat);
        },

        isLiveFirst(): boolean {
            return this.chatSortMode === ChatSortMode.LiveChat;
        },

        sortPopularFirst() {
            this.chatSortMode = ChatSortMode.PopularFirst;
            this.chatSortFn = ChatMessageSorter.GetSortFn(ChatSortMode.PopularFirst);
        },

        isPopularFirst(): boolean {
            return this.chatSortMode === ChatSortMode.PopularFirst;
        },

        async initWebsocket() {
            const handler = (data) => {
                if ("message" in data) {
                    this.handleNewMessage(data);
                }
            };
            this.ws.subscribe(handler);
        },

        async loadMessages() {
            this.messages = await ChatAPI.getMessages(this.streamId);
        },

        handleNewMessage(data) {
            data["replies"] = []; // go serializes this empty list as `null`
            if (data["replyTo"].Valid) {
                console.log("🌑 received reply", data);
                this.messages.pushReply(data);
            } else {
                console.log("🌑 received message", data);
                this.messages.pushMessage(data);
            }
        },
    } as AlpineComponent;
}
