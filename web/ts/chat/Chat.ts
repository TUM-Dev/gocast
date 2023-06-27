import { deregisterTimeWatcher, getPlayers, registerTimeWatcher } from "../TUMLiveVjs";

enum ShowMode {
    Messages,
    Polls,
}

export class Chat {
    readonly userId: number;
    readonly userName: string;
    readonly admin: boolean;
    readonly streamId: number;

    popUpWindow: Window;
    chatReplayActive: boolean;
    messages: ChatMessage[];
    focusedMessageId?: number;
    startTime: Date;
    liveNowTimestamp: Date;
    pollHistory: object[];
    showMode: ShowMode;
    orderByLikes: boolean;

    preprocessors: ((m: ChatMessage) => ChatMessage)[] = [
        (m: ChatMessage) => {
            return { ...m, isGrayedOut: this.chatReplayActive && !this.orderByLikes };
        },
    ];

    filterPredicate: (m: ChatMessage) => boolean = (m) => {
        return !this.admin && !m.visible && m.userId != this.userId;
    };

    private timeWatcherCallBackFunction: () => void;

    constructor(activateChatReplay: boolean) {
        this.orderByLikes = false;
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
        const currentTime = getPlayers()[0].currentTime();
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
            if (Math.trunc(playerTime) === Math.trunc(getPlayers()[0].duration())) {
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

    // patchMessage adds the message to the list of messages at the position it should appear in based on the send time.
    patchMessage(m: ChatMessage): void {
        if (this.filterPredicate(m)) {
            this.messages = this.messages.filter((m2) => m2.ID !== m.ID);
            return;
        }

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
            }
        }

        window.dispatchEvent(new CustomEvent("reorder"));
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

export type ChatMessage = {
    ID: number;
    admin: boolean;

    userId: number;
    message: string;
    name: string;
    color: string;

    reactions: ChatReaction[];
    aggregatedReactions: ChatReactionGroup[]; // is generated in frontend

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

type ChatReaction = {
    userID: number;
    username: string;
    emoji: string;
};

type ChatReactionGroup = {
    emoji: string;
    emojiName: string;
    names: string[];
    namesPretty: string;
    reactions: ChatReaction[];
    hasReacted: boolean;
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
