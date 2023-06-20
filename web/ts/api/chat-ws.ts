import { WebsocketConnection } from "../utilities/ws";

enum ChatMessageType {
    Message = "message",
    Delete = "delete",
    Approve = "approve",
    Retract = "retract",
    Resolve = "resolve",
    ReactTo = "react_to",
}

export abstract class SocketConnections {
    static ws: WebsocketConnection = new WebsocketConnection("chat/12845");
}

SocketConnections.ws.subscribe();

export type NewChatMessage = {
    msg: string;
    anonymous: boolean;
    replyTo: number;
    addressedTo: number[];
};

export class ChatWebsocketConnection {
    private readonly ws: WebsocketConnection;

    constructor(ws: WebsocketConnection) {
        this.ws = ws;
    }

    sendMessage(msg: NewChatMessage) {
        return this.ws.send({
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
        return this.ws.send({ payload: { type, id } });
    }
}
