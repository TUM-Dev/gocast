import { startPoll } from "../watch";

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

    start() {
        startPoll(
            this.question,
            // @ts-ignore
            this.options.map(({ answer }) => answer),
        );
        this.reset();
    }

    async load() {
        const res = await fetch("/api/chat/" + this.streamId + "/active-poll");
        this.activePoll = await res.json();
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
