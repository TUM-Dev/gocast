import { sendMessage } from "../watch";
import { getCurrentWordPositions } from "./misc";
import { ChatMessage } from "./Chat";

export class NewChatMessage {
    message: string;
    reply: Reply;
    anonymous: boolean;
    addressedTo: ChatUser[];

    constructor() {
        this.anonymous = false;
        this.clear();
    }

    send(): void {
        sendMessage(this);
        this.clear();
    }

    clear(): void {
        this.message = "";
        this.reply = undefined;
        this.addressedTo = [];
    }

    setReply(m: ChatMessage) {
        this.reply = {
            message: m,
            id: m.ID,
        };
    }

    cancelReply() {
        this.reply = undefined;
    }

    showReply(messageId: number): boolean {
        return this.reply === undefined || this.reply.id !== messageId;
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
            " " +
            this.message.substring(pos[1] + this.message.substring(pos[0], pos[1]).length);

        chatInput.focus();

        this.addressedTo.push(user);
    }
}

export type ChatUser = {
    id: number;
    name: string;
};

type Reply = {
    message: ChatMessage;
    id: number;
};
