import { WebsocketConnection } from "../utilities/ws";
import { Realtime } from "../socket";

export enum PollMessageType {
    StartPoll = "start_poll",
    SubmitPollOptionVote = "submit_poll_option_vote",
    CloseActivePoll = "close_active_poll",
}

export class PollWebsocketConnection {
    private readonly ws: WebsocketConnection;

    constructor(ws: WebsocketConnection) {
        this.ws = ws;
    }

    startPoll(question: string, pollAnswers: string[]) {
        return this.ws.send({
            payload: {
                type: PollMessageType.StartPoll,
                question,
                pollAnswers,
            },
        });
    }

    submitPollOptionVote(pollOptionId: number) {
        return this.ws.send({
            payload: {
                type: PollMessageType.SubmitPollOptionVote,
                pollOptionId,
            },
        });
    }

    closeActivePoll() {
        return this.ws.send({ payload: { type: PollMessageType.CloseActivePoll } });
    }
}
