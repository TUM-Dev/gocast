import { getCurrentWordPositions } from "./misc";

export class ChatUserList {
    subset: object[];
    streamId: number;
    currIndex: number;
    private all: object[];

    constructor(streamId: number) {
        this.all = this.subset = [];
        this.streamId = streamId;
        this.currIndex = 0;
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

        this.currIndex = 0; // reset index on show
        setTimeout(() => {
            document.getElementById("chatInput").blur();
            document.getElementById("userList").focus();
        }, 100); // wait until alpine has shown the userList element
    }

    next() {
        this.currIndex = (this.currIndex + 1) % this.subset.length;
    }

    prev() {
        this.currIndex = (this.currIndex - 1) % this.subset.length;
    }

    getSelected() {
        console.log("hello");
        return this.subset[this.currIndex];
    }
}
