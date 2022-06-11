import { NewChatMessage } from "./NewChatMessage";
import { ChatUserList } from "./ChatUserList";
import { EmojiList } from "./EmojiList";
import { Poll } from "./Poll";
import { registerTimeWatcher } from "../TUMLiveVjs";

export class Chat {
    readonly userId: number;
    readonly userName: string;
    readonly admin: boolean;
    readonly streamId: number;

    windows: Window[];
    orderByLikes: boolean;
    disconnected: boolean;
    current: NewChatMessage;
    messages: ChatMessage[];
    focusedMessageId?: number;
    users: ChatUserList;
    emojis: EmojiList;
    startTime: Date;
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
            return { ...m, isGrayedOut: !this.orderByLikes };
        },
    ];

    constructor(isAdminOfCourse: boolean, streamId: number, startTime: string, userId: number, userName: string) {
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
        this.startTime = new Date(startTime);
        this.focusedMessageId = -1;
        this.windows = [window];
        this.grayOutMessagesAfterPlayerTime = this.grayOutMessagesAfterPlayerTime.bind(this);
        registerTimeWatcher(this.grayOutMessagesAfterPlayerTime);
        window.addEventListener("beforeunload", () => {
            this.windows.forEach((window) => window.close());
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

        if (this.windows.length > 2) return;
        const height = window.innerHeight * 0.8;
        const popUpWindow = window.open(
            `/w/${courseSlug}/${streamID}/chat/popup`,
            "_blank",
            `popup=yes,width=420,innerWidth=420,height=${height},innerHeight=${height}`,
        );

        popUpWindow.addEventListener("beforeunload", (_) => {
            this.windows = this.windows.filter((window) => window !== popUpWindow);
        });

        popUpWindow.dispatchEvent(
            new CustomEvent("chat-messages-updated", { detail: JSON.parse(JSON.stringify(this.messages)) }),
        );

        this.windows.push(popUpWindow);
    }

    /**
     * Grays out all messages that were not sent at the same time in the livestream.
     * @param playerTime time offset of current player time w.r.t. video start in seconds
     */
    grayOutMessagesAfterPlayerTime(playerTime: number): void {
        if (this.orderByLikes) {
            if (this.messages.some((message) => message.isGrayedOut)) {
                this.messages.forEach((message) => (message.isGrayedOut = false));
                this.notifyMessagesUpdate(this.messages);
            } else return;
        }

        //TODO revert:  const referenceTime = new Date(this.startTime);
        const referenceTime = new Date("Mon Jun 03 2022 12:28:30");
        referenceTime.setSeconds(referenceTime.getSeconds() + playerTime);

        const grayOutCondition = (CreatedAt: string) => {
            const dateCreatedAt = new Date(CreatedAt);
            return dateCreatedAt > referenceTime;
        };

        this.messages.forEach((message: ChatMessage) => (message.isGrayedOut = grayOutCondition(message.CreatedAt)));

        const messagesNotGrayedOut = this.messages.filter(
            (message: ChatMessage) => !message.isGrayedOut && message.visible,
        );

        this.focusedMessageId = messagesNotGrayedOut.pop()?.ID;
        this.notifyMessagesUpdate(this.messages);
    }

    isMessageToBeFocused = (index: number) => this.messages[index].ID === this.focusedMessageId;

    private addMessage(m: ChatMessage) {
        this.preprocessors.forEach((f) => (m = f(m)));
        this.messages.push(m);
    }

    private notifyMessagesUpdate(messages: ChatMessage[]) {
        this.windows.forEach((window: Window) =>
            window.dispatchEvent(
                new CustomEvent("chat-messages-updated", { detail: JSON.parse(JSON.stringify(messages)) }),
            ),
        );
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

    replies: object[];
    replyTo: object; // e.g.{Int64:0, Valid:false}

    addressedTo: number[];
    resolved: boolean;
    visible: true;
    deleted: boolean;
    isGrayedOut: boolean;

    CreatedAt: string;
    DeletedAt: string;
    UpdatedAt: string;
};
