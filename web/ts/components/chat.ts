import { AlpineComponent } from "./alpine-component";
import { ChatAPI, ChatMessage, ChatMessageArray, ChatReaction } from "../api/chat";
import { ChatMessageSorter, ChatSortMode } from "../chat/ChatMessageSorter";
import { ChatMessagePreprocessor } from "../chat/ChatMessagePreprocessor";
import { ChatWebsocketConnection, SocketConnections } from "../api/chat-ws";
import { User } from "../api/users";

export function chatContext(streamId: number, user: User): AlpineComponent {
    SocketConnections.chat = new ChatWebsocketConnection(`chat/${streamId}`);

    return {
        streamId: streamId as number,
        user: user as User,

        chatSortMode: ChatSortMode.LiveChat,
        chatSortFn: ChatMessageSorter.GetSortFn(ChatSortMode.LiveChat),
        messages: ChatMessageArray.EmptyArray() as ChatMessageArray,

        preprocessors: [ChatMessagePreprocessor.AggregateReactions, ChatMessagePreprocessor.AddressedToCurrentUser],

        async init() {
            Promise.all([this.loadMessages(), this.initWebsocket()]).then(() => {
                this.messages.forEach((msg, _) => this.preprocessors.forEach((f) => f(msg, this.user)));
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
                } else if ("delete" in data) {
                    this.handleDelete(data.delete);
                } else if ("resolve" in data) {
                    this.handleResolve(data.resolve);
                } else if ("approve" in data) {
                    this.handleApprove(data.chat);
                } else if ("reactions" in data) {
                    this.handleReaction(data);
                }
            };
            SocketConnections.chat.subscribe(handler);
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
                this.preprocessors.forEach((f) => f(msg, this.user));
                this.messages.pushMessage(msg);
            }
        },

        handleDelete(messageId: number) {
            this.messages.delete({ ID: messageId });
        },

        handleResolve(messageId: number) {
            this.messages.resolve({ ID: messageId });
        },

        handleApprove(msg: ChatMessage) {
            this.messages.approve(msg);
        },

        handleReaction(reaction: { reactions: number; payload: ChatReaction[] }) {
            console.log("🌑 received reaction", reaction);
            this.messages.setReaction(reaction, this.user);
        },
    } as AlpineComponent;
}
