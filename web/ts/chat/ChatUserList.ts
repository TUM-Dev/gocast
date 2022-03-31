import { getCurrentWordPositions } from "./misc";

export class ChatUserList {
    subset: object[];
    streamId: number;
    currIndex: number;
    valid: boolean;
    private all: object[];

    constructor(streamId: number) {
        this.all = this.subset = [];
        this.streamId = streamId;
        this.currIndex = 0;
        this.valid = false;
    }

    async LoadAll(): Promise<object[]> {
        return fetch(`/api/chat/${this.streamId}/users`).then((res) => res.json());
    }

    isValid(): boolean {
        return this.subset.length >= 0 && this.valid;
    }

    filterUsers(message: string, cursorPos: number) {
        const pos = getCurrentWordPositions(message, cursorPos);
        if (pos[0] === 0 && pos[1] === 0) {
            // substring(0,0) returns ''
            pos[1] = 1;
        }

        const currentWord = message.substring(pos[0], pos[1]);
        console.log(message);
        console.log(currentWord);
        if (message === "" || !currentWord.startsWith("@")) {
            this.subset = [];
            this.valid = false;
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
        this.valid = true;
        this.currIndex = 0; // reset index on show

        // only focus if there are users to choose from
        if (this.subset.length !== 0) {
            setTimeout(() => {
                document.getElementById("chatInput").blur();
                document.getElementById("userList").focus();
            }, 50); // wait until alpine has shown the userList element
        }
    }

    clear() {
        this.subset = [];
        this.valid = false;
    }

    next(e) {
        e.preventDefault();
        this.currIndex = (this.currIndex + 1) % this.subset.length;
    }

    prev(e) {
        e.preventDefault();
        this.currIndex = (this.currIndex - 1) % this.subset.length;
    }

    onKeyUp(e: KeyboardEvent) {
        if (e.keyCode === 8) {
            const chatInput: HTMLInputElement = document.getElementById("chatInput") as HTMLInputElement;
            chatInput.focus();
            chatInput.value = chatInput.value.substring(0, chatInput.value.length - 1);
            this.filterUsers(chatInput.value, chatInput.selectionStart);
        }
    }

    getSelected() {
        return this.subset[this.currIndex];
    }
}
