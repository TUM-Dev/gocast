import { AlpineComponent } from "./alpine-component";
import { ChatAPI, ChatMessage, ChatMessageArray, ChatReaction } from "../api/chat";
import { WebsocketConnection } from "../chat/ws";
import { ChatMessageSorter, ChatSortMode } from "../chat/ChatMessageSorter";
import { ChatMessagePreprocessor } from "../chat/ChatMessagePreprocessor";
import { ChatWebsocketConnection } from "../api/chat-ws";

export function chatContext(streamId: number, userId: number): AlpineComponent {
    return {
        streamId: streamId as number,
        userId: userId as number,

        chatSortMode: ChatSortMode.LiveChat,
        chatSortFn: ChatMessageSorter.GetSortFn(ChatSortMode.LiveChat),
        messages: ChatMessageArray.EmptyArray() as ChatMessageArray,

        preprocessors: [ChatMessagePreprocessor.AggregateReactions],

        ws: new ChatWebsocketConnection(`chat/${streamId}`),

        async init() {
            Promise.all([this.loadMessages(), this.initWebsocket()]).then(() => {
                this.messages.forEach((msg, _) => this.preprocessors.forEach((f) => f(msg, this.userId)));
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
                } else if ("reactions" in data) {
                    this.handleReaction(data);
                }
            };
            this.ws.subscribe(handler);
        },

        async loadMessages() {
            this.messages = await ChatAPI.getMessages(this.streamId);
        },

        handleNewMessage(msg: ChatMessage) {
            msg["replies"] = []; // go serializes this empty list as `null`
            if (msg["replyTo"].Valid) {
                console.log("🌑 received reply", msg);
                this.messages.pushReply(msg);
            } else {
                console.log("🌑 received message", msg);
                this.messages.pushMessage(msg);
            }
        },

        handleReaction(reaction: { reactions: number; payload: ChatReaction[] }) {
            console.log("🌑 received reaction", reaction);
            this.messages.setReaction(reaction, this.userId);
        },
    } as AlpineComponent;
}
