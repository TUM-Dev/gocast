import { NewChatMessage } from "./NewChatMessage";
import { ChatUserList } from "./ChatUserList";
import { EmojiList } from "./EmojiList";
import { Poll } from "./Poll";
import { registerTimeWatcher, deregisterTimeWatcher, getPlayer } from "../TUMLiveVjs";
import { create } from "nouislider";

export class Chat {
    readonly userId: number;
    readonly userName: string;
    readonly admin: boolean;
    readonly streamId: number;

    popUpWindow: Window;
    chatReplayActive: boolean;
    orderByLikes: boolean;
    disconnected: boolean;
    current: NewChatMessage;
    messages: ChatMessage[];
    focusedMessageId?: number;
    users: ChatUserList;
    emojis: EmojiList;
    startTime: Date;
    liveNowTimestamp: Date;
    poll: Poll;

    preprocessors: ((m: ChatMessage) => ChatMessage)[] = [
        (m: ChatMessage) => {
            if (m.addressedTo.find((uId) => uId === this.userId) !== undefined) {
                m.message = m.message.replaceAll(
                    "@" + this.userName,
                    "<span class = 'text-sky-800 bg-sky-200 text-xs dark:text-indigo-200 dark:bg-indigo-800 p-1 rounded'>" +
                        "@" +
                        this.userName +
                        "</span>",
                );
            }
            return m;
        },
        (m: ChatMessage) => {
            return { ...m, isGrayedOut: this.chatReplayActive && !this.orderByLikes };
        },
        (m: ChatMessage) => {
            return { ...m, renderVersion: 0 };
        },
    ];

    private timeWatcherCallBackFunction: () => void;

    constructor(
        isAdminOfCourse: boolean,
        streamId: number,
        startTime: string,
        liveNowTimestamp: string,
        userId: number,
        userName: string,
        activateChatReplay: boolean,
    ) {
        this.orderByLikes = false;
        this.disconnected = false;
        this.current = new NewChatMessage();
        this.admin = isAdminOfCourse;
        this.users = new ChatUserList(streamId);
        this.emojis = new EmojiList();
        this.messages = [];
        this.streamId = streamId;
        this.userId = userId;
        this.userName = userName;
        this.poll = new Poll(streamId);
        this.startTime = Date.parse(startTime) ? new Date(startTime) : null;
        this.liveNowTimestamp = Date.parse(liveNowTimestamp) ? new Date(liveNowTimestamp) : null;
        this.focusedMessageId = -1;
        this.popUpWindow = null;
        this.grayOutMessagesAfterPlayerTime = this.grayOutMessagesAfterPlayerTime.bind(this);
        this.deregisterPlayerTimeWatcher = this.deregisterPlayerTimeWatcher.bind(this);
        this.registerPlayerTimeWatcher = this.registerPlayerTimeWatcher.bind(this);
        this.activateChatReplay = this.activateChatReplay.bind(this);
        this.deactivateChatReplay = this.deactivateChatReplay.bind(this);
        if (activateChatReplay) {
            this.activateChatReplay();
        } else {
            this.deactivateChatReplay();
        }
        window.addEventListener("beforeunload", () => {
            this.popUpWindow?.close();
        });
    }

    async loadMessages() {
        this.messages = [];
        fetchMessages(this.streamId).then((messages) => {
            messages.forEach((m) => this.addMessage(m));
        });
    }

    sortMessages() {
        this.messages.sort((m1, m2) => {
            if (this.orderByLikes) {
                if (m1.likes === m2.likes) {
                    return m2.ID - m1.ID; // same amount of likes -> newer messages up
                }
                return m2.likes - m1.likes; // more likes -> up
            } else {
                return m1.ID < m2.ID ? -1 : 1; // newest messages last
            }
        });
    }

    onMessage(e) {
        this.addMessage(e.detail);
    }

    onDelete(e) {
        this.messages.find((m) => m.ID === e.detail.delete).deleted = true;
    }

    onLike(e) {
        this.messages.find((m) => m.ID === e.detail.likes).likes = e.detail.num;
    }

    onResolve(e) {
        this.messages.find((m) => m.ID === e.detail.resolve).resolved = true;
    }

    onReply(e) {
        this.messages.find((m) => m.ID === e.detail.replyTo.Int64).replies.push(e.detail);
    }

    onNewPoll(e) {
        if (!this.current.anonymous) {
            this.poll.result = null;
            this.poll.activePoll = { ...e.detail, selected: null };
        }
    }

    onPopUpMessagesUpdated(e) {
        const messagesToUpdate: ChatMessage[] = e.detail;
        if (messagesToUpdate) {
            this.messages = [];
            messagesToUpdate.forEach((message) => this.messages.push(message));
        }
    }

    onGrayedOutUpdated(e) {
        this.messages.find((m) => m.ID === e.detail.ID).isGrayedOut = e.detail.isGrayedOut;
    }

    onFocusUpdated(e) {
        this.focusedMessageId = e.detail.ID;
    }

    onPollOptionVotesUpdate(e) {
        this.poll.updateVotes(e.detail);
    }

    onPollOptionResult(e) {
        this.poll.activePoll = null;
        this.poll.result = e.detail;
    }

    onSubmit() {
        if (this.emojis.isValid()) {
            window.dispatchEvent(new CustomEvent("chatenter"));
        } else if (this.users.isValid()) {
            this.current.addAddressee(this.users.getSelected());
            this.users.clear();
        } else {
            this.current.send();
        }
    }

    onInputKeyUp(e) {
        if (this.emojis.isValid()) {
            let event = "";
            switch (e.keyCode) {
                case 9: {
                    event = "emojiselectiontriggered";
                    break;
                }
                case 38: {
                    event = "emojiarrowup";
                    break;
                }
                case 40: {
                    event = "emojiarrowdown";
                    break;
                }
                default: {
                    this.emojis.getEmojisForMessage(e.target.value, e.target.selectionStart);
                    return;
                }
            }
            window.dispatchEvent(new CustomEvent(event));
        } else if (this.users.isValid()) {
            switch (e.keyCode) {
                case 38 /* UP */: {
                    this.users.prev();
                    break;
                }
                case 40 /* DOWN */: {
                    this.users.next();
                    break;
                }
                default: {
                    this.users.filterUsers(e.target.value, e.target.selectionStart);
                    return;
                }
            }
        } else {
            this.users.filterUsers(e.target.value, e.target.selectionStart);
            this.emojis.getEmojisForMessage(e.target.value, e.target.selectionStart);
        }
    }

    getInputPlaceHolder(): string {
        if (this.disconnected) {
            return "Reconnecting to chat...";
        }
        if (this.current.replyTo === 0) {
            return "Send a message";
        } else {
            return "Reply [escape to cancel]";
        }
    }

    openChatPopUp(courseSlug: string, streamID: number) {
        // multiple popup chat windows seem to trigger Alpine.js exceptions
        // which is probably caused by a race condition during the update of the
        // chat messages array with the custom event
        if (this.popUpWindow) {
            this.popUpWindow.focus();
            return;
        }
        const height = window.innerHeight * 0.8;
        const popUpWindow = window.open(
            `/w/${courseSlug}/${streamID}/chat/popup`,
            "_blank",
            `popup=yes,width=420,innerWidth=420,height=${height},innerHeight=${height}`,
        );

        popUpWindow.addEventListener("beforeunload", (_) => {
            this.popUpWindow = null;
        });

        popUpWindow.addEventListener("chatinitialized", () => {
            this.messages.forEach((message) => {
                const type: MessageUpdateType = "chatupdategrayedout";
                const payload: MessageUpdate = { ID: message.ID, isGrayedOut: message.isGrayedOut };
                popUpWindow.dispatchEvent(new CustomEvent(type, { detail: payload }));
            });
            const type: MessageUpdateType = "chatupdatefocus";
            const payload: FocusUpdate = { ID: this.focusedMessageId };
            popUpWindow.dispatchEvent(new CustomEvent(type, { detail: payload }));
        });

        this.popUpWindow = popUpWindow;
    }

    /**
     * registers for updates regarding current player time
     */
    registerPlayerTimeWatcher(): void {
        this.timeWatcherCallBackFunction = registerTimeWatcher(this.grayOutMessagesAfterPlayerTime);
    }

    /**
     * deregisters updates regarding current player time
     */
    deregisterPlayerTimeWatcher(): void {
        if (this.timeWatcherCallBackFunction) {
            deregisterTimeWatcher(this.timeWatcherCallBackFunction);
            this.timeWatcherCallBackFunction = null;
        }
    }

    activateChatReplay(): void {
        this.chatReplayActive = true;
        const currentTime = getPlayer().currentTime();
        //force update of message focus and grayedOut state
        this.focusedMessageId = -1;
        this.grayOutMessagesAfterPlayerTime(currentTime);
        this.registerPlayerTimeWatcher();
    }

    deactivateChatReplay(): void {
        this.chatReplayActive = false;
        this.deregisterPlayerTimeWatcher();
        this.messages.map((message) =>
            this.notifyMessagesUpdate("chatupdategrayedout", { ID: message.ID, isGrayedOut: false }),
        );
    }

    /**
     * Grays out all messages that were not sent at the same time in the livestream.
     * @param playerTime time offset of current player time w.r.t. video start in seconds
     */
    grayOutMessagesAfterPlayerTime(playerTime: number): void {
        // create new Date instance for a deep copy
        const referenceTime = new Date(this.liveNowTimestamp ?? this.startTime);
        if (!referenceTime) {
            this.deactivateChatReplay();
        }
        referenceTime.setSeconds(referenceTime.getSeconds() + playerTime);

        const grayOutCondition = (CreatedAt: string) => {
            if (Math.trunc(playerTime) === Math.trunc(getPlayer().duration())) {
                return false;
            }

            const dateCreatedAt = new Date(CreatedAt);
            return dateCreatedAt > referenceTime;
        };

        const grayedOutUpdates: GrayedOutUpdate[] = [];
        const messagesNotGrayedOut = [];
        this.messages.forEach((message: ChatMessage) => {
            if (!message.replyTo.Valid) {
                const shouldBeGrayedOut = grayOutCondition(message.CreatedAt);
                if (message.isGrayedOut !== shouldBeGrayedOut) {
                    grayedOutUpdates.push({ ID: message.ID, isGrayedOut: shouldBeGrayedOut });
                }

                message.replies.forEach((reply) =>
                    grayedOutUpdates.push({ ID: reply.ID, isGrayedOut: shouldBeGrayedOut }),
                );

                if (!shouldBeGrayedOut && message.visible) {
                    messagesNotGrayedOut.push(message);
                }
            }
        });

        const focusedMessageId: number = messagesNotGrayedOut.pop()?.ID ?? this.messages[0]?.ID;
        const focusedMessageChanged = this.focusedMessageId !== focusedMessageId;

        if (focusedMessageChanged) {
            this.notifyMessagesUpdate("chatupdatefocus", { ID: focusedMessageId });
        }

        grayedOutUpdates.forEach((grayedOutUpdate) =>
            this.notifyMessagesUpdate("chatupdategrayedout", grayedOutUpdate),
        );
    }

    isMessageToBeFocused = (index: number) => this.messages[index].ID === this.focusedMessageId;

    patchMessage(m: ChatMessage): void {
        this.preprocessors.forEach((f) => (m = f(m)));

        const newMessageCreatedAt = Date.parse(m.CreatedAt);

        for (let i = 0; i <= this.messages.length; i++) {
            if (i == this.messages.length) {
                this.messages.push(m);
                break;
            }

            const createdAt = Date.parse(this.messages[i].CreatedAt);
            if (createdAt === newMessageCreatedAt) {
                const newRenderVersion = this.messages[i].renderVersion + 1;
                this.messages.splice(i, 1, { ...m, renderVersion: newRenderVersion });
                break;
            } else if (createdAt > newMessageCreatedAt) {
                this.messages.splice(i, 0, m);
                break;
            }
        }
        console.log(this.messages);
    }

    private addMessage(m: ChatMessage) {
        this.preprocessors.forEach((f) => (m = f(m)));
        this.messages.push(m);
    }

    private notifyMessagesUpdate(type: MessageUpdateType, payload: MessageUpdate) {
        [window, this.popUpWindow].forEach((window: Window) => {
            window?.dispatchEvent(new CustomEvent(type, { detail: payload }));
        });
    }
}

export async function fetchMessages(streamId: number): Promise<ChatMessage[]> {
    return await fetch("/api/chat/" + streamId + "/messages")
        .then((res) => res.json())
        .then((messages) => {
            return messages;
        });
}

type ChatMessage = {
    ID: number;
    admin: boolean;

    message: string;
    name: string;
    color: string;

    liked: false;
    likes: number;

    replies: ChatMessage[];
    replyTo: Reply; // e.g.{Int64:0, Valid:false}

    addressedTo: number[];
    resolved: boolean;
    visible: true;
    deleted: boolean;
    isGrayedOut: boolean;
    renderVersion: number;

    CreatedAt: string;
    DeletedAt: string;
    UpdatedAt: string;
};

type Reply = {
    Int64: number;
    Valid: boolean;
};

type GrayedOutUpdate = {
    ID: number;
    isGrayedOut: boolean;
};

type FocusUpdate = {
    ID: number;
};

type MessageUpdate = FocusUpdate | GrayedOutUpdate;
type MessageUpdateType = "chatupdatefocus" | "chatupdategrayedout";
