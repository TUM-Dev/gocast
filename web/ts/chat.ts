import { Emoji, TopEmojis } from "top-twitter-emojis-map";
import { sendMessage } from "./watch";

export class ChatUserList {
    all: object[];
    subset: object[];
    streamId: string;

    constructor(streamId: string) {
        this.all = [];
        this.subset = [];
        this.streamId = streamId;
    }

    async LoadAll() {
        await fetch(`/api/chat/${this.streamId}/users`)
            .then((res) => res.json())
            .then((users: object[]) => {
                this.all = users;
                this.subset = users;
            });
    }

    filterUsers(message: string, cursorPos: number): string[] {
        const pos = getCurrentWordPositions(message, cursorPos);
        const currentWord = message.substring(pos[0], pos[1]);
        console.log(currentWord);
        if (message === "" || !currentWord.startsWith("@")) {
            this.subset = [];
            return;
        }
        if (currentWord === "@") {
            this.LoadAll().then(() => {
                /* */
            }); // Load on '@'
        } else {
            const input = currentWord.substring(1);
            // @ts-ignore
            this.subset = this.all.filter((user) => user.name.startsWith(input));
        }
    }
}

export class ChatMessage {
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

    parse(): void {
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
        this.addressedTo.push(user);
    }
}

export class Chat {
    orderByLikes: boolean;
    current: ChatMessage;
    messages: object[];
    admin: boolean;
    users: ChatUserList;

    constructor(isAdminOfCourse: boolean, streamId: string) {
        this.orderByLikes = false;
        this.current = new ChatMessage();
        this.messages = [];
        this.admin = isAdminOfCourse;
        this.users = new ChatUserList(streamId);
    }
}

class ChatUser {
    id: number;
    name: string;

    constructor(id: number, name: string) {
        this.id = id;
        this.name = name;
    }
}

export function initChat(isAdminOfCourse: boolean, streamId: string) {
    return new Chat(isAdminOfCourse, streamId);
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

export function sortMessages(messages): void {
    messages.sort((m1, m2) => {
        if (orderByLikes) {
            if (m1.likes === m2.likes) {
                return m2.id - m1.id; // same amount of likes -> newer messages up
            }
            return m2.likes - m1.likes; // more likes -> up
        } else {
            return m1.ID < m2.ID ? -1 : 1; // newest messages last
        }
    });
}

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
