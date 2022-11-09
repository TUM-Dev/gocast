import { sendMessage } from "../watch";
import { getCurrentWordPositions } from "./misc";
import {markdownEditor, MarkdownEditor} from "../markdown-editor";

export class NewChatMessage {
    message: string;
    replyTo: number;
    anonymous: boolean;
    addressedTo: ChatUser[];
    markdownEditor: MarkdownEditor;

    constructor() {
        this.message = "";
        this.replyTo = 0;
        this.anonymous = false;
        this.addressedTo = [];
        this.markdownEditor = markdownEditor({headings: false, images: false});
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
