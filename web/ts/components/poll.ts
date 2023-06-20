import { AlpineComponent } from "./alpine-component";
import { SocketConnections } from "../api/chat-ws";
import { PollWebsocketConnection } from "../api/poll-ws";

export function pollContext(): AlpineComponent {
    return {
        activePoll: undefined as object,
        result: undefined as object,
        showCreateUI: false as boolean,
        question: "" as string,
        options: [] as object[],

        ws: new PollWebsocketConnection(SocketConnections.ws),

        async init() {
            Promise.all([this.initWebsocket()]).then(() => {
                console.log("hello");
            });
        },

        async initWebsocket() {
            const handler = (data) => {
                if ("pollOptions" in data) {
                    this.handleNewPoll(data);
                }
            };
            SocketConnections.ws.addHandler(handler);
        },

        handleNewPoll(data: object) {
            console.log("🌑 starting new poll", data);
            this.result = null;
            this.activePoll = { ...data, selected: null };
        },
    } as AlpineComponent;
}
