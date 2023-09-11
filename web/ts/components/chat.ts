import { AlpineComponent } from "./alpine-component";
import { ChatAPI, ChatMessage, ChatMessageArray, ChatReaction } from "../api/chat";
import { ChatMessageSorter, ChatSortMode } from "../chat/ChatMessageSorter";
import { ChatMessagePreprocessor } from "../chat/ChatMessagePreprocessor";
import { ChatWebsocketConnection, SocketConnections } from "../api/chat-ws";
import { User } from "../api/users";
import { Tunnel } from "../utilities/tunnels";
import Alpine from "alpinejs";
import { ToggleableElement } from "../utilities/ToggleableElement";
import { VideoJsPlayer } from "video.js";
import { registerTimeWatcher, deregisterTimeWatcher } from "../video/watchers";

export function chatContext(streamId: number, user: User, isRecording: boolean): AlpineComponent {
    return {
        streamId: streamId as number,
        user: user as User,
        isRecording: isRecording,

        chatSortMode: ChatSortMode.LiveChat,
        chatSortFn: ChatMessageSorter.GetSortFn(ChatSortMode.LiveChat),
        messages: ChatMessageArray.EmptyArray() as ChatMessageArray,

        ws: new ChatWebsocketConnection(SocketConnections.ws),

        chatBoxEl: document.getElementById("chat-box") as HTMLInputElement,

        status: true,
        serverMessage: {},
        unreadMessages: false,

        attachedPlayer: undefined as VideoJsPlayer,
        streamStart: undefined as Date,
        replay: ChatReplay.get(),

        showSortSelect: new ToggleableElement(),

        preprocessors: [ChatMessagePreprocessor.AggregateReactions, ChatMessagePreprocessor.AddressedToCurrentUser],

        __initpromise: null as Promise<never>,
        async init() {
            this.__initpromise = Promise.all([this.loadMessages(), this.initWebsocket()]);
        },

        afterInitNotPopout(player: VideoJsPlayer, streamStart: string) {
            if (this.isRecording) {
                this.__initpromise.then(() => {
                    this.preprocessors.push(ChatMessagePreprocessor.GrayOut);
                    this.messages.forEach((msg, _) => this.preprocessors.forEach((f) => f(msg, this.user)));
                    this.replay.activate(player, this.updateGrayedOut.bind(this));
                });
            } else {
                Alpine.nextTick(() => this.scrollToBottom());
            }
            this.attachedPlayer = player;
            this.streamStart = new Date(streamStart);
        },

        afterInitPopout() {
            this.__initpromise.then(() => {
                // eslint-disable-next-line @typescript-eslint/no-empty-function
                this.deactivateReplay = () => {}; // for the popout chat this is simply a NOOP
                this.messages.forEach((msg, _) => this.preprocessors.forEach((f) => f(msg, this.user)));
                Alpine.nextTick(() => this.scrollToBottom());
            });
        },

        sortLiveFirst() {
            this.deactivateReplay();
            this.chatSortMode = ChatSortMode.LiveChat;
            this.chatSortFn = ChatMessageSorter.GetSortFn(ChatSortMode.LiveChat);
            Alpine.nextTick(() => this.scrollToBottom());
        },

        isLiveFirst(): boolean {
            return this.chatSortMode === ChatSortMode.LiveChat;
        },

        sortPopularFirst() {
            this.deactivateReplay();
            this.chatSortMode = ChatSortMode.PopularFirst;
            this.chatSortFn = ChatMessageSorter.GetSortFn(ChatSortMode.PopularFirst);
            Alpine.nextTick(() => this.scrollToTop());
        },

        isPopularFirst(): boolean {
            return this.chatSortMode === ChatSortMode.PopularFirst;
        },

        toggleReplay() {
            if (this.isReplaying()) this.deactivateReplay();
            else {
                this.messages.forEach((msg: ChatMessage, _) => (msg.isGrayedOut = true));
                this.replay.activate(this.attachedPlayer, this.updateGrayedOut.bind(this));
            }
        },

        isReplaying(): boolean {
            return this.replay.isActivated();
        },

        activateReplay() {
            this.preprocessors.push(ChatMessagePreprocessor.GrayOut);
            this.messages.forEach((msg: ChatMessage, _) => (msg.isGrayedOut = true));
            this.replay.activate(this.attachedPlayer, this.updateGrayedOut.bind(this));
        },

        deactivateReplay() {
            this.preprocessors.pop(); // Remove GrayOut
            this.messages.forEach((msg: ChatMessage, _) => (msg.isGrayedOut = false));
            this.replay.deactivate(this.attachedPlayer);
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

        scrollToMessage(id: number) {
            document
                .getElementById("chat-message-" + id)
                .scrollIntoView({ behavior: "smooth", block: "end", inline: "nearest" });
        },

        updateGrayedOut(t: number) {
            let next;
            const referenceTime = new Date(this.streamStart);
            referenceTime.setSeconds(referenceTime.getSeconds() + t);

            const grayOutCondition = (createdAt: string) => {
                return (
                    Math.trunc(t) !== Math.trunc(this.attachedPlayer.duration()) && new Date(createdAt) > referenceTime
                );
            };

            this.messages.forEach((msg: ChatMessage, _) => {
                msg.isGrayedOut = grayOutCondition(msg.CreatedAt);
                if (!msg.isGrayedOut) next = msg;
            });

            if (next) this.scrollToMessage(next.ID);
            else this.scrollToTop();
        },

        handleDelete(messageId: number) {
            this.messages.delete({ ID: messageId });
        },

        handleResolve(messageId: number) {
            this.messages.resolve({ ID: messageId });
        },

        handleApprove(msg: ChatMessage) {
            this.preprocessors.forEach((f) => f(msg, this.user));
            if (msg.replyTo.Valid) this.messages.approveReply(msg);
            else this.messages.approve(msg);
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

class ChatReplay {
    private static instance: ChatReplay;
    static get(): ChatReplay {
        if (this.instance == null) this.instance = new ChatReplay();
        return this.instance;
    }

    private activated: boolean;
    private callback: () => void;

    constructor() {
        this.activated = false;
    }

    isActivated(): boolean {
        return this.activated;
    }

    deactivate(player: VideoJsPlayer) {
        this.activated = false;
        deregisterTimeWatcher(player, this.callback);
    }

    activate(player: VideoJsPlayer, callback: (t: number) => void) {
        this.activated = true;
        this.callback = registerTimeWatcher(player, callback);
    }
}
