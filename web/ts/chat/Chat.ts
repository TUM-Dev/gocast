import { getPlayers } from "../TUMLiveVjs";
import { deregisterTimeWatcher, registerTimeWatcher } from "../video/watchers";

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
    orderByLikes: boolean;

    preprocessors: ((m: ChatMessage) => ChatMessage)[] = [
        (m: ChatMessage) => {
            return { ...m, isGrayedOut: this.chatReplayActive && !this.orderByLikes };
        },
    ];

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

    onGrayedOutUpdated(e) {
        this.messages.find((m) => m.ID === e.detail.ID).isGrayedOut = e.detail.isGrayedOut;
    }

    onFocusUpdated(e) {
        this.focusedMessageId = e.detail.ID;
    }

    /**
     * registers for updates regarding current player time
     */
    registerPlayerTimeWatcher(): void {
        this.timeWatcherCallBackFunction = registerTimeWatcher(getPlayers()[0], this.grayOutMessagesAfterPlayerTime);
    }

    /**
     * deregisters updates regarding current player time
     */
    deregisterPlayerTimeWatcher(): void {
        if (this.timeWatcherCallBackFunction) {
            deregisterTimeWatcher(getPlayers()[0], this.timeWatcherCallBackFunction);
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

    private notifyMessagesUpdate(type: MessageUpdateType, payload: MessageUpdate) {
        [window, this.popUpWindow].forEach((window: Window) => {
            window?.dispatchEvent(new CustomEvent(type, { detail: payload }));
        });
    }
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
