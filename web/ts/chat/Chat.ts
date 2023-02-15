import { NewChatMessage } from "./NewChatMessage";
import { ChatUserList } from "./ChatUserList";
import { EmojiList } from "./EmojiList";
import { Poll } from "./Poll";
import { deregisterTimeWatcher, getPlayers, registerTimeWatcher } from "../TUMLiveVjs";
import { EmojiPicker } from "./EmojiPicker";
import { TopEmojis } from "top-twitter-emojis-map";

const MAX_NAMES_IN_REACTION_TITLE = 2;

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
    pollHistory: object[];
    showMode: ShowMode;

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
        (m: ChatMessage) => {
            m.aggregatedReactions = (m.reactions || [])
                .reduce((res: ChatReactionGroup[], reaction: ChatReaction) => {
                    let group: ChatReactionGroup = res.find((r) => r.emojiName === reaction.emoji);
                    if (group === undefined) {
                        group = {
                            emoji: TopEmojis.find((e) => e.short_names.includes(reaction.emoji)).emoji,
                            emojiName: reaction.emoji,
                            reactions: [],
                            names: [],
                            namesPretty: "",
                            hasReacted: reaction.userID === this.userId,
                        };
                        res.push(group);
                    } else if (reaction.userID == this.userId) {
                        group.hasReacted = true;
                    }

                    group.names.push(reaction.username);
                    group.reactions.push(reaction);
                    return res;
                }, [])
                .map((group) => {
                    if (group.names.length === 0) {
                        // Nobody
                        group.namesPretty = `Nobody reacted with ${group.emojiName}`;
                    } else if (group.names.length == 1) {
                        // One Person
                        group.namesPretty = `${group.names[0]} reacted with ${group.emojiName}`;
                    } else if (group.names.length == MAX_NAMES_IN_REACTION_TITLE + 1) {
                        // 1 person more than max allowed
                        group.namesPretty = `${group.names
                            .slice(0, MAX_NAMES_IN_REACTION_TITLE)
                            .join(", ")} and one other reacted with ${group.emojiName}`;
                    } else if (group.names.length > MAX_NAMES_IN_REACTION_TITLE) {
                        // at least 2 more than max allowed
                        group.namesPretty = `${group.names.slice(0, MAX_NAMES_IN_REACTION_TITLE).join(", ")} and ${
                            group.names.length - MAX_NAMES_IN_REACTION_TITLE
                        } others reacted with ${group.emojiName}`;
                    } else {
                        // More than 1 Person but less than MAX_NAMES_IN_REACTION_TITLE
                        group.namesPretty = `${group.names.slice(0, group.names.length - 1).join(", ")} and ${
                            group.names[group.names.length - 1]
                        } reacted with ${group.emojiName}`;
                    }
                    return group;
                });
            m.aggregatedReactions.sort(
                (a, b) => EmojiPicker.getEmojiIndex(a.emojiName) - EmojiPicker.getEmojiIndex(b.emojiName),
            );
            return m;
        },
    ];

    filterPredicate: (m: ChatMessage) => boolean = (m) => {
        return !this.admin && !m.visible && m.userId != this.userId;
    };

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
        this.showMode = ShowMode.Messages;
        this.pollHistory = [];
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

    initEmojiPicker(id: string): EmojiPicker {
        return new EmojiPicker(id);
    }

    async loadMessages() {
        this.messages = [];
        fetchMessages(this.streamId).then((messages) => {
            messages.forEach((m) => this.addMessage(m));
        });
    }

    async loadPollHistory() {
        this.pollHistory = [];
        fetch(`/api/chat/${this.streamId}/polls`)
            .then((r) => r.json())
            .then((polls) => (this.pollHistory = polls));
    }

    showMessages(set = false): boolean {
        this.showMode = set ? ShowMode.Messages : this.showMode;
        return this.showMode == ShowMode.Messages;
    }

    showPolls(set = false): boolean {
        this.showMode = set ? ShowMode.Polls : this.showMode;
        return this.showMode == ShowMode.Polls;
    }

    sortMessages() {
        this.messages = [...this.messages].sort((m1, m2) => {
            if (this.orderByLikes) {
                const m1LikeReactionGroup = m1.aggregatedReactions.find(
                    (r) => r.emojiName === EmojiPicker.LikeEmojiName,
                );
                const m1Likes = m1LikeReactionGroup ? m1LikeReactionGroup.reactions.length : 0;

                const m2LikeReactionGroup = m2.aggregatedReactions.find(
                    (r) => r.emojiName === EmojiPicker.LikeEmojiName,
                );
                const m2Likes = m2LikeReactionGroup ? m2LikeReactionGroup.reactions.length : 0;

                if (m1Likes === m2Likes) {
                    return m2.ID - m1.ID; // same amount of likes -> newer messages up
                }
                return m2Likes - m1Likes; // more likes -> up
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

    onReaction(e) {
        const m = this.messages.find((m) => m.ID === e.detail.reactions);
        m.reactions = e.detail.payload;
        this.patchMessage(m);
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
        // @ts-ignore
        const id = this.pollHistory.length > 0 ? this.pollHistory[0].ID + 1 : 1;
        this.pollHistory.unshift({
            // @ts-ignore
            ID: id,
            question: e.detail.question,
            options: e.detail.pollOptionResults,
        });
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

    private addMessage(m: ChatMessage) {
        this.preprocessors.forEach((f) => (m = f(m)));

        if (!this.filterPredicate(m)) {
            this.messages.push(m);
        }
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

    userId: number;
    message: string;
    name: string;
    color: string;

    liked: false;
    likes: number;
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
