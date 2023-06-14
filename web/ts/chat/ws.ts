import { MessageHandlerFn, Realtime } from "../socket";

enum WSMessageType {
    Message = "message",
    Like = "like",
    Delete = "delete",
    StartPoll = "start_poll",
    SubmitPollOptionVote = "submit_poll_option_vote",
    CloseActivePoll = "close_active_poll",
    Approve = "approve",
    Retract = "retract",
    Resolve = "resolve",
}

export class WebsocketConnection {
    private readonly channel: string;

    connected: boolean;

    constructor(channel: string) {
        this.channel = channel;
    }

    async subscribe(handler: MessageHandlerFn) {
        Realtime.get()
            .subscribeChannel(this.channel, handler)
            .then(() => (this.connected = true));
    }
}

type Event = {
    name: string;
    callback: (data: object) => void;
};
