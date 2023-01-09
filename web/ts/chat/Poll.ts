import { WebsocketConnection, WSMessageType } from "../WebsocketConnection";

export class Poll {
    readonly streamId: number;

    activePoll: object;
    result: object;
    showCreateUI: boolean;
    question: string;
    options: object[];

    constructor(streamId: number) {
        this.streamId = streamId;
        this.activePoll = null;
        this.result = null;
        this.reset();
    }

    start(ws: WebsocketConnection) {
        const args = {
            question: this.question,
            // @ts-ignore
            pollAnswers: this.options.map(({ answer }) => answer),
        };
        ws.sendCustomMessage(WSMessageType.StartPoll, args);
        this.reset();
    }

    async load() {
        this.activePoll = await fetch("/api/chat/" + this.streamId + "/active-poll")
            .then((res) => {
                if (!res.ok) {
                    throw Error(res.statusText);
                }
                return res;
            })
            .then((res) => res.json())
            .catch((err) => undefined); // return undefined if error
    }

    addEmptyOption() {
        this.options.push({ answer: "" });
    }

    removeOption(option: object) {
        this.options = this.options.filter((o) => o !== option);
    }

    updateVotes(vote: PollVote) {
        // @ts-ignore
        this.activePoll.pollOptions =
            // @ts-ignore
            this.activePoll.pollOptions.map((pollOption) =>
                // @ts-ignore
                pollOption.ID === vote.pollOptionId ? { ...pollOption, votes: vote.votes } : pollOption,
            );
    }

    private reset() {
        this.question = "";
        this.options = [{ answer: "Yes" }, { answer: "No" }];
        this.showCreateUI = false;
    }
}

type PollVote = {
    pollOptionId: number;
    votes: number;
};
