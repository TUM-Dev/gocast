import { Emoji, TopEmojis } from "top-twitter-emojis-map";
import { sendMessage } from "./watch";

class Chat {
    readonly userId: number;
    readonly userName: string;
    readonly admin: boolean;
    readonly streamId: number;

    orderByLikes: boolean;
    current: NewChatMessage;
    messages: ChatMessage[];
    users: ChatUserList;

    messageParsers: ((m: ChatMessage) => ChatMessage)[] = [
        (m: ChatMessage) => {
            if (m.addressedTo.find((uId) => uId === this.userId) !== undefined) {
                m.message = m.message.replaceAll(
                    "@" + this.userName,
                    "<span class = 'text-sky-600'>" + "@" + this.userName + "</span>",
                );
            }
            return m;
        },
    ];

    constructor(isAdminOfCourse: boolean, streamId: number, userId: number, userName: string) {
        this.orderByLikes = false;
        this.current = new NewChatMessage();
        this.admin = isAdminOfCourse;
        this.users = new ChatUserList(streamId);
        this.messages = [];
        this.streamId = streamId;
        this.userId = userId;
        this.userName = userName;
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
                // @ts-ignore
                if (m1.likes === m2.likes) {
                    // @ts-ignore
                    return m2.id - m1.id; // same amount of likes -> newer messages up
                }
                // @ts-ignore
                return m2.likes - m1.likes; // more likes -> up
            } else {
                // @ts-ignore
                return m1.ID < m2.ID ? -1 : 1; // newest messages last
            }
        });
    }

    onMessage(e) {
        this.addMessage(e.detail);
    }

    onDelete(e) {
        // @ts-ignore
        this.messages.find((m) => m.ID === e.detail.delete).deleted = true;
    }

    onLike(e) {
        // @ts-ignore
        this.messages.find((m) => m.ID === e.detail.likes).likes = e.detail.num;
    }

    onResolve(e) {
        // @ts-ignore
        this.messages.find((m) => m.ID === e.detail.resolve).resolved = true;
    }

    onReply(e) {
        // @ts-ignore
        this.messages.find((m) => m.ID === e.detail.replyTo.Int64).replies.push(e.detail);
    }

    onInputKeyup(e) {
        let event = "";
        switch (e.keyCode) {
            case 9: {
                event = "emojiselectiontriggered";
                break;
            }
            case 13: {
                event = "chatenter";
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
                console.log(e.target.value);
                this.users.filterUsers(e.target.value, e.target.selectionStart);
                return getEmojisForMessage(e.target.value, e.target.selectionStart).then((emojis) => {
                    window.dispatchEvent(new CustomEvent("emojisuggestions", { detail: emojis }));
                    return;
                });
            }
        }
        window.dispatchEvent(new CustomEvent(event));
    }

    private addMessage(m: ChatMessage) {
        this.messageParsers.forEach((f) => (m = f(m)));
        this.messages.push(m);
    }
}

export class NewChatMessage {
    message: string;
    replyTo: number;
    anonymous: boolean;
    addressedTo: ChatUser[];

    constructor() {
        this.message = "";
        this.replyTo = 0;
        this.anonymous = false;
        this.addressedTo = [];
    }

    send(): void {
        sendMessage(this);
        this.clear();
    }

    clear(): void {
        this.message = "";
        this.replyTo = 0;
        this.addressedTo = [];
    }

    isEmpty(): boolean {
        return this.message === "";
    }

    parse(): void {
        // remove unused @username's from addressee list
        this.addressedTo = this.addressedTo.filter((user) => this.message.includes(`@${user.name}`));
    }

    addAddressee(user: ChatUser): void {
        const chatInput: HTMLInputElement = document.getElementById("chatInput") as HTMLInputElement;
        const pos = getCurrentWordPositions(this.message, chatInput.selectionStart);

        // replace message with username e.g. 'Hello @Ad' to 'Hello @Admin':
        this.message =
            this.message.substring(0, pos[0]) +
            this.message.substring(pos[0], pos[1]).replace(/@(\w)*/, "@" + user.name) +
            this.message.substring(pos[1] + this.message.substring(pos[0], pos[1]).length);

        chatInput.focus();

        this.addressedTo.push(user);
    }
}

class ChatUserList {
    subset: object[];
    streamId: number;
    private all: object[];

    constructor(streamId: number) {
        this.all = this.subset = [];
        this.streamId = streamId;
    }

    async LoadAll(): Promise<object[]> {
        return fetch(`/api/chat/${this.streamId}/users`).then((res) => res.json());
    }

    isValid(): boolean {
        return this.subset.length > 0;
    }

    filterUsers(message: string, cursorPos: number) {
        const pos = getCurrentWordPositions(message, cursorPos);
        if (pos[0] === 0 && pos[1] === 0) {
            // substring(0,0) returns ''
            pos[1] = 1;
        }

        const currentWord = message.substring(pos[0], pos[1]);
        if (message === "" || !currentWord.startsWith("@")) {
            this.subset = [];
            return;
        }

        if (currentWord === "@") {
            // load users on '@'
            this.LoadAll().then((users) => {
                this.all = this.subset = users;
            });
        } else {
            const input = currentWord.substring(1);
            // @ts-ignore
            this.subset = this.all.filter((user) => user.name.startsWith(input));
        }
    }
}

export function initChat(isAdminOfCourse: boolean, streamId: number, userId: number, userName: string) {
    return { c: new Chat(isAdminOfCourse, streamId, userId, userName) };
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
    resolved: false;
    visible: true;

    CreatedAt: string;
    DeletedAt: string;
    UpdatedAt: string;
};

type ChatUser = {
    ID: number;
    name: string;
};

export async function fetchMessages(streamId: number): Promise<ChatMessage[]> {
    return await fetch("/api/chat/" + streamId + "/messages")
        .then((res) => res.json())
        .then((messages) => {
            return messages;
        });
}

/*
    Scroll to the bottom of the 'chatBox'
 */
export function scrollToBottom() {
    document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight;
}

/*
    Scroll to top of the 'chatBox'
 */
export function scrollToTop() {
    document.getElementById("chatBox").scrollTo({ top: 0, behavior: "smooth" });
}

export function findEmojisForInput(input: string): Emoji[] {
    return TopEmojis.filter((emoji) => {
        return emoji.short_names.some((key) => key.startsWith(input));
    }).slice(0, 7);
}

/**
 * get currently typed word based on position in the input.
 * e.g.: "hello{cursor} world" => "hello" ([0, 4])
 */
function getCurrentWordPositions(input: string, cursorPos: number): [number, number] {
    const cursorStart = cursorPos;
    while (cursorPos > 0 && input.charAt(cursorPos - 1) !== " ") {
        cursorPos--;
    }
    return [cursorPos, cursorStart];
}

export async function getEmojisForMessage(message: string, cursorPos: number): Promise<Emoji[]> {
    const pos = getCurrentWordPositions(message, cursorPos);
    const currentWord = message.substring(pos[0], pos[1]);
    if (!currentWord.startsWith(":") || currentWord.length < 2) {
        return [];
    }
    return findEmojisForInput(currentWord.substring(1));
}

export function insertEmoji(emoji: Emoji) {
    const chatInput: HTMLInputElement = document.getElementById("chatInput") as HTMLInputElement;
    const pos = getCurrentWordPositions(chatInput.value, chatInput.selectionStart);
    // send new message to alpine
    window.dispatchEvent(
        new CustomEvent("setmessage", {
            detail: chatInput.value.substring(0, pos[0]) + emoji.emoji + " " + chatInput.value.substring(pos[1]),
        }),
    );
    chatInput.focus();
    chatInput.selectionStart = pos[0] + emoji.emoji.length + 1; // +1 for space
    chatInput.selectionEnd = pos[0] + emoji.emoji.length + 1;
    // notify alpine to remove emoji suggestions
    window.dispatchEvent(new CustomEvent("emojisinserted"));
}

let orderByLikes = false; // sorting by likes or by time

export function setOrder(obl: boolean) {
    orderByLikes = obl;
}

export function shouldScroll(): boolean {
    if (orderByLikes) {
        return false; // only scroll if sorting by time
    }
    const c = document.getElementById("chatBox");
    return c.scrollHeight - c.scrollTop <= c.offsetHeight;
}

export function showNewMessageIndicator() {
    window.dispatchEvent(new CustomEvent("messageindicator", { detail: { show: true } }));
}

export function scrollChat() {
    if (orderByLikes) {
        return; // only scroll if sorting by time
    }
    const c = document.getElementById("chatBox");
    c.scrollTop = c.scrollHeight;
}

export function scrollToLatestMessage() {
    const c = document.getElementById("chatBox");
    c.scrollTo({ top: c.scrollHeight, behavior: "smooth" });
    window.dispatchEvent(new CustomEvent("messageindicator", { detail: { show: false } }));
}

export function showDisconnectedMsg() {
    if (document.getElementById("disconnectMsg") !== null) {
        document.getElementById("disconnectMsg").classList.remove("hidden");
    }
}

export function hideDisconnectedMsg() {
    if (document.getElementById("disconnectMsg") !== null) {
        document.getElementById("disconnectMsg").classList.add("hidden");
    }
}

export function openChatPopUp(courseSlug: string, streamID: number) {
    const height = window.innerHeight * 0.8;
    window.open(
        `/w/${courseSlug}/${streamID}/chat/popup`,
        "_blank",
        `popup=yes,width=420,innerWidth=420,height=${height},innerHeight=${height}`,
    );
}

export function messageDateToString(date: string) {
    const d = new Date(date);
    return ("0" + d.getHours()).slice(-2) + ":" + ("0" + d.getMinutes()).slice(-2);
}
