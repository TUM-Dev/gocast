import { WebsocketConnection } from "../chat/ws";

enum ChatMessageType {
    Message = "message",
    Delete = "delete",
    Approve = "approve",
    Retract = "retract",
    Resolve = "resolve",
    ReactTo = "react_to",
}

enum PollMessageType {
    StartPoll = "start_poll",
    SubmitPollOptionVote = "submit_poll_option_vote",
    CloseActivePoll = "close_active_poll",
}

export class ChatWebsocketConnection extends WebsocketConnection {
    deleteMessage(id: number) {
        return this.sendIDMessage(id, ChatMessageType.Delete);
    }

    resolveMessage(id: number) {
        return this.sendIDMessage(id, ChatMessageType.Resolve);
    }

    approveMessage(id: number) {
        return this.sendIDMessage(id, ChatMessageType.Approve);
    }

    retractMessage(id: number) {
        return this.sendIDMessage(id, ChatMessageType.Retract);
    }

    private sendIDMessage(id: number, type: ChatMessageType) {
        return this.send({ payload: { type, id } });
    }
}
