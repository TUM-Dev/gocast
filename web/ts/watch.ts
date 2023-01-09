import { SeekLogger } from "./TUMLiveVjs";
import { WSMessageType, WebsocketConnection } from "./WebsocketConnection";

export const watchPage = (streamID: number) => ({
    streamID: streamID,
    showChat: false,
    showShare: false,
    seekLogger: null,
    showBookmarks: false,
    showShortcuts: false,
    ws: null,

    async init() {
        this.showChat = this.$persist(false).as("chatOpen");
        this.seekLogger = new SeekLogger(this.streamID);
        this.ws = new WebsocketConnection(this.streamID);

        this.seekLogger.attach();
        await this.ws.subscribe();

        window.dispatchEvent(new CustomEvent("connected"));
    },

    reactToMessage: function (id: number, reaction: string) {
        this.ws.sendCustomIDMessage(id, WSMessageType.ReactTo, { reaction });
    },

    deleteMessage: function (id: number) {
        this.ws.sendCustomIDMessage(id, WSMessageType.Delete);
    },

    resolveMessage: function (id: number) {
        this.ws.sendCustomIDMessage(id, WSMessageType.Resolve);
    },

    approveMessage: function (id: number) {
        this.ws.sendCustomIDMessage(id, WSMessageType.Approve);
    },

    retractMessage: function (id: number) {
        this.ws.sendCustomIDMessage(id, WSMessageType.Retract);
    },

    closeActivePoll: function () {
        this.ws.sendCustomMessage(WSMessageType.CloseActivePoll);
    },

    submitPollOptionVote: function (pollOptionId: number) {
        this.ws.sendCustomMessage(WSMessageType.SubmitPollOptionVote, { pollOptionId });
    },

    startPoll: function (question: string, pollAnswers: string[]) {
        this.ws.sendCustomMessage(WSMessageType.StartPoll, { question, pollAnswers });
    },

    onShift(e) {
        if (document.activeElement.tagName !== "INPUT" && document.activeElement.tagName !== "TEXTAREA") {
            switch (e.key) {
                case "?": {
                    this.toggleShortcuts();
                }
            }
        }
    },

    toggleShortcuts() {
        const el = document.getElementById("shortcuts-help-modal");
        if (el !== undefined) {
            if (el.classList.contains("hidden")) {
                el.classList.remove("hidden");
            } else {
                el.classList.add("hidden");
            }
        }
    },

    getWS() {
        console.log("this.ws");
        console.log(this.ws);
        return this.ws;
    },
});

export function initChatScrollListener() {
    const chatBox = document.getElementById("chatBox") as HTMLDivElement;
    if (!chatBox) {
        return;
    }
    chatBox.addEventListener("scroll", function (e) {
        if (chatBox.scrollHeight - chatBox.scrollTop === chatBox.offsetHeight) {
            window.dispatchEvent(new CustomEvent("messageindicator", { detail: { show: false } }));
        }
    });
}

export function createServerMessage(msg) {
    const serverElem = document.createElement("div");
    switch (msg["type"]) {
        case "error":
            serverElem.classList.add("text-danger", "font-semibold");
            break;
        case "info":
            serverElem.classList.add("text-4");
            break;
        case "warn":
            serverElem.classList.add("text-warn", "font-semibold");
            break;
    }
    serverElem.classList.add("text-sm", "p-2");
    serverElem.innerText = msg["server"];
    return serverElem;
}

export function getPollOptionWidth(pollOptions, pollOption) {
    const minWidth = 1;
    const maxWidth = 100;
    const maxVotes = Math.max(...pollOptions.map(({ votes: v }) => v));

    if (pollOption.votes == 0) return `${minWidth.toString()}%`;

    const fractionOfMax = pollOption.votes / maxVotes;
    const fractionWidth = minWidth + fractionOfMax * (maxWidth - minWidth);
    return `${Math.ceil(fractionWidth).toString()}%`;
}
