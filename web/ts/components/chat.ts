import { AlpineComponent } from "./alpine-component";
import { ChatAPI, ChatMessage, ChatMessageArray, ChatReaction } from "../api/chat";
import { ChatMessageSorter, ChatSortMode } from "../chat/ChatMessageSorter";
import { ChatMessagePreprocessor } from "../chat/ChatMessagePreprocessor";
import { ChatWebsocketConnection, SocketConnections } from "../api/chat-ws";
import { User } from "../api/users";
import { Tunnel } from "../utilities/tunnels";
import Alpine from "alpinejs";
import { ToggleableElement } from "../utilities/ToggleableElement";

export function chatContext(streamId: number, user: User): AlpineComponent {
    return {
        streamId: streamId as number,
        user: user as User,

        chatSortMode: ChatSortMode.LiveChat,
        chatSortFn: ChatMessageSorter.GetSortFn(ChatSortMode.LiveChat),
        messages: ChatMessageArray.EmptyArray() as ChatMessageArray,

        ws: new ChatWebsocketConnection(SocketConnections.ws),

        chatBoxEl: document.getElementById("chat-box") as HTMLInputElement,

        status: true,
        serverMessage: {},
        unreadMessages: false,

        showSortSelect: new ToggleableElement(),

        preprocessors: [ChatMessagePreprocessor.AggregateReactions, ChatMessagePreprocessor.AddressedToCurrentUser],

        async init() {
            Promise.all([this.loadMessages(), this.initWebsocket()]).then(() => {
                this.messages.forEach((msg, _) => this.preprocessors.forEach((f) => f(msg, this.user)));
                Alpine.nextTick(() => this.scrollToBottom());
            });
        },

        sortLiveFirst() {
            this.chatSortMode = ChatSortMode.LiveChat;
            this.chatSortFn = ChatMessageSorter.GetSortFn(ChatSortMode.LiveChat);
            Alpine.nextTick(() => this.scrollToBottom());
        },

        isLiveFirst(): boolean {
            return this.chatSortMode === ChatSortMode.LiveChat;
        },

        sortPopularFirst() {
            this.chatSortMode = ChatSortMode.PopularFirst;
            this.chatSortFn = ChatMessageSorter.GetSortFn(ChatSortMode.PopularFirst);
            Alpine.nextTick(() => this.scrollToTop());
        },

        isPopularFirst(): boolean {
            return this.chatSortMode === ChatSortMode.PopularFirst;
        },

        reactToMessage(id: number, reaction: string) {
            return this.ws.reactToMessage(id, reaction);
        },

        setReply(message: ChatMessage) {
            Tunnel.reply.add({ message });
        },

        setStatus(status = false) {
            this.status = status;
        },

        hasServerMessage() {
            return Object.keys(this.serverMessage).length > 0;
        },

        hideServerMessage() {
            this.serverMessage = {};
        },

        isConnected() {
            return this.status;
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
                } else if ("retract" in data) {
                    this.handleRetract(data.chat);
                } else if ("reactions" in data) {
                    this.handleReaction(data);
                } else if ("server" in data) {
                    this.handleServerMessage(data);
                }
            };
            SocketConnections.ws.addHandler(handler);
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
                const sib = this.scrollIsBottom();

                console.log("🌑 received message", msg);
                this.preprocessors.forEach((f) => f(msg, this.user));
                this.messages.pushMessage(msg);

                const cb = () => {
                    if (sib) this.scrollToBottom();
                    else {
                        this.unreadMessages = true;
                    }
                };
                Alpine.nextTick(cb);
            }
        },

        scrollIsBottom(delta = 128) {
            return (
                Math.abs(this.chatBoxEl.scrollHeight - this.chatBoxEl.clientHeight - this.chatBoxEl.scrollTop) < delta
            );
        },

        scrollToBottom() {
            this.chatBoxEl.scrollTo({ top: this.chatBoxEl.scrollHeight, behavior: "smooth" });
            this.unreadMessages = false;
        },

        scrollToTop() {
            this.chatBoxEl.scrollTo({ top: 0, behavior: "smooth" });
        },

        handleDelete(messageId: number) {
            this.messages.delete({ ID: messageId });
        },

        handleResolve(messageId: number) {
            this.messages.resolve({ ID: messageId });
        },

        handleApprove(msg: ChatMessage) {
            this.preprocessors.forEach((f) => f(msg, this.user));
            this.messages.approve(msg);
        },

        handleRetract(msg: ChatMessage) {
            this.messages.retract(msg, this.user.isAdmin);
        },

        handleReaction(reaction: { reactions: number; payload: ChatReaction[] }) {
            console.log("🌑 received reaction", reaction);
            this.messages.setReaction(reaction, this.user);
        },

        handleServerMessage(msg: { server: string; type: string }) {
            console.log("🌑 received server message", msg);
            this.serverMessage = { msg: msg.server };
        },

        deleteMessage(id: number) {
            return this.ws.deleteMessage(id);
        },

        approveMessage(id: number) {
            return this.ws.approveMessage(id);
        },

        retractMessage(id: number) {
            return this.ws.retractMessage(id);
        },

        resolveMessage(id: number) {
            return this.ws.resolveMessage(id);
        },
    } as AlpineComponent;
}
