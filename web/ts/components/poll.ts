import { AlpineComponent } from "./alpine-component";
import { SocketConnections } from "../api/chat-ws";
import { PollMessageType, PollWebsocketConnection } from "../api/poll-ws";
import { ChatAPI, Poll, PollOption } from "../api/chat";
import { ToggleableElement } from "../utilities/ToggleableElement";

export function pollContext(streamId: number): AlpineComponent {
    return {
        streamId: streamId,

        // current poll
        activePoll: null as Poll,

        // create new poll
        showCreateUI: new ToggleableElement(),
        newPoll: new NewPoll(),

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
            return this.activePoll !== null && this.activePoll !== undefined && this.activePoll.ID !== -1;
        },

        closeActivePoll() {
            this.ws.closeActivePoll();
            this.activePoll = null;
        },

        startPoll() {
            this.showCreateUI.toggle(false);
            this.ws.startPoll(this.newPoll.question, this.newPoll.options);
        },

        cancelPoll() {
            this.showCreateUI.toggle(false);
            this.newPoll.reset();
        },

        submitPollOptionVote(pollOptionId: number) {
            this.ws.submitPollOptionVote(pollOptionId);
            this.activePoll.submitted = this.activePoll.selected;
            this.activePoll.selected = null;
        },

        handleNewPoll(data: object) {
            console.log("🌑 starting new poll", data);
            this.activePoll = Object.assign(new Poll(), data);
            console.log(this.activePoll);
        },

        handleParticipation(vote: PollVote) {
            this.activePoll.options = this.activePoll.options.map((opt) =>
                opt.ID === vote.pollOptionId ? { ...opt, votes: vote.votes } : opt,
            );
        },

        handleClosePoll(result) {
            console.log("🌑 close poll", result);
            this.activePoll = null;

            this.history.unshift(
                Object.assign(new Poll(), {
                    ID: this.history.length > 0 ? this.history[0].ID + 1 : 1,
                    question: result.question,
                    options: result.options,
                }),
            );
        },
    } as AlpineComponent;
}

class NewPoll {
    question: string;
    options: object[];

    constructor() {
        this.reset();
    }

    isValid(): boolean {
        // @ts-ignore
        return this.question.length === 0 || this.options.some(({ answer }) => answer.length === 0);
    }

    addEmptyOption() {
        this.options.push({ answer: "" });
    }

    onlyOneOption() {
        return this.options.length === 1;
    }

    reset() {
        this.question = "";
        this.options = [{ answer: "Yes" }, { answer: "No" }];
    }
}

type PollVote = {
    pollOptionId: number;
    votes: number;
};
