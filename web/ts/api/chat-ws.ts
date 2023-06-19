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

export abstract class SocketConnections {
    static chat: ChatWebsocketConnection;
}

export type NewChatMessage = {
    msg: string;
    anonymous: boolean;
    replyTo: number;
    addressedTo: number[];
};

export class ChatWebsocketConnection extends WebsocketConnection {
    sendMessage(msg: NewChatMessage) {
        return this.send({
            payload: {
                type: ChatMessageType.Message,
                ...msg,
            },
        });
    }

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
