import { AlpineComponent } from "./alpine-component";
import { SocketConnections } from "../api/chat-ws";
import { PollMessageType, PollWebsocketConnection } from "../api/poll-ws";
import { ChatAPI, Poll } from "../api/chat";

export function pollContext(streamId: number): AlpineComponent {
    return {
        streamId: streamId,

        // current poll
        activePoll: undefined as Poll,
        result: undefined as object,

        // create new poll
        showCreateUI: false as boolean,
        question: "" as string,
        options: [] as object[],

        // poll history
        history: [] as Poll[],

        ws: new PollWebsocketConnection(SocketConnections.ws),

        async init() {
            Promise.all([this.loadActivePoll(), this.loadHistory(), this.initWebsocket()]);
        },

        async initWebsocket() {
            const handler = (data) => {
                const type = data.type;
                if (type === PollMessageType.StartPoll) {
                    this.handleNewPoll(data);
                } else if (type === PollMessageType.SubmitPollOptionVote) {
                    this.handleParticipation(data);
                } else if (type === PollMessageType.CloseActivePoll) {
                    this.handleClosePoll(data);
                }
            };
            SocketConnections.ws.addHandler(handler);
        },

        async loadActivePoll() {
            this.activePoll = await ChatAPI.getActivePoll(this.streamId);
        },

        async loadHistory() {
            this.history = await ChatAPI.getPolls(this.streamId);
        },

        hasActivePoll(): boolean {
            return this.activePoll.ID !== -1;
        },

        closeActivePoll() {
            this.ws.closeActivePoll();
            this.activePoll = null;
        },

        submitPollOptionVote(pollOptionId: number) {
            this.ws.submitPollOptionVote(pollOptionId);
            this.activePoll.submitted = this.activePoll.selected;
            this.activePoll.selected = null;
        },

        handleNewPoll(data: object) {
            console.log("🌑 starting new poll", data);
            this.result = null;
            this.activePoll = { ...data, selected: null };
        },

        handleParticipation(vote: PollVote) {
            this.activePoll.options = this.activePoll.options.map((opt) =>
                opt.ID === vote.pollOptionId ? { ...opt, votes: vote.votes } : opt,
            );
        },

        handleClosePoll(result) {
            console.log("🌑 close poll", result);
            this.activePoll = null;
            this.result = result;

            this.history.unshift({
                ID: this.history.length > 0 ? this.history[0].ID + 1 : 1,
                question: result.question,
                options: result.pollOptionResults,
            });
        },
    } as AlpineComponent;
}

type PollVote = {
    pollOptionId: number;
    votes: number;
};
