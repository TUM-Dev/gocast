import { Realtime } from "./socket";
import { NewChatMessage } from "./chat/NewChatMessage";
import { scrollChat, shouldScroll, showNewMessageIndicator } from "./chat";
import { createServerMessage } from "./watch";

const SCROLL_DELAY = 100; // delay before scrolling to bottom to make sure chat is rendered

export enum WSMessageType {
    Message = "message",
    Delete = "delete",
    StartPoll = "start_poll",
    SubmitPollOptionVote = "submit_poll_option_vote",
    CloseActivePoll = "close_active_poll",
    Approve = "approve",
    Retract = "retract",
    Resolve = "resolve",
    ReactTo = "react_to",
}

export class WebsocketConnection {
    private readonly chatChannel: string;
    private events: Event[] = [
        { name: "viewers", callback: (data) => triggerViewersEvent(data) },
        { name: "live", callback: (data) => triggerStreamEnded(data) },
        { name: "server", callback: (data) => handleServerMessage(data) },
        { name: "message", callback: (data) => triggerOnMessage(data) },
        { name: "pollOptions", callback: (data) => triggerDataEvent("chatnewpoll", data) },
        { name: "pollOptionId", callback: (data) => triggerDataEvent("polloptionvotesupdate", data) },
        { name: "pollOptionResults", callback: (data) => triggerDataEvent("polloptionresult", data) },
        { name: "likes", callback: (data) => triggerDataEvent("chatlike", data) },
        { name: "delete", callback: (data) => triggerDataEvent("chatdelete", data) },
        { name: "resolve", callback: (data) => triggerDataEvent("chatresolve", data) },
        { name: "approve", callback: (data) => triggerDataEvent("chatapprove", data) },
        { name: "retract", callback: (data) => triggerDataEvent("chatretract", data) },
        { name: "title", callback: (data) => triggerDataEvent("titleupdate", data) },
        { name: "description", callback: (data) => triggerDataEvent("descriptionupdate", data) },
        { name: "reactions", callback: (data) => triggerDataEvent("chatreactions", data) },
    ];

    constructor(streamId: number) {
        this.chatChannel = `chat/${streamId}`;
    }

    async subscribe() {
        await Realtime.get().subscribeChannel(this.chatChannel, this.getMessageHandler());
    }

    sendMessage(current: NewChatMessage) {
        return Realtime.get().send(this.chatChannel, {
            payload: {
                type: WSMessageType.Message,
                msg: current.message,
                anonymous: current.anonymous,
                replyTo: current.replyTo,
                addressedTo: current.addressedTo.map((u) => u.id),
            },
        });
    }

    sendCustomIDMessage(id: number, type: WSMessageType, optArgs: object = {}) {
        return Realtime.get().send(this.chatChannel, {
            payload: {
                type: type,
                id: id,
                ...optArgs,
            },
        });
    }

    sendCustomMessage(type: WSMessageType, optArgs: object = {}) {
        return Realtime.get().send(this.chatChannel, {
            payload: {
                type: type,
                ...optArgs,
            },
        });
    }

    private getMessageHandler(): (object) => void {
        return (data: object) => {
            this.events.forEach((e) => {
                if (e.name in data) {
                    e.callback(data);
                }
            });
        };
    }
}

type Event = {
    name: string;
    callback: (data: object) => void;
};

const triggerDataEvent = (type, data) => window.dispatchEvent(new CustomEvent(type, { detail: data }));

const triggerViewersEvent = (data) =>
    window.dispatchEvent(new CustomEvent("viewers", { detail: { viewers: data["viewers"] } }));

const triggerStreamEnded = (data) =>
    data["live"] ? window.location.reload() : window.dispatchEvent(new CustomEvent("streamended"));

function triggerOnMessage(data) {
    data["replies"] = []; // go serializes this empty list as `null`
    // replies
    if (data["replyTo"].Valid) {
        // reply
        window.dispatchEvent(new CustomEvent("chatreply", { detail: data }));
    } else {
        // message
        const scroll = shouldScroll();
        window.dispatchEvent(new CustomEvent("chatmessage", { detail: data }));
        if (scroll) {
            setTimeout(scrollChat, SCROLL_DELAY);
        } else {
            showNewMessageIndicator();
        }
    }
}

function handleServerMessage(data) {
    const scroll = shouldScroll();
    const serverElem = createServerMessage(data);
    document.getElementById("chatBox").appendChild(serverElem);
    if (scroll) {
        setTimeout(scrollChat, SCROLL_DELAY);
    } else {
        showNewMessageIndicator();
    }
}

export async function startWebsocket(streamId: number) {
    const conn = new WebsocketConnection(streamId);
    await conn.subscribe();
    window.dispatchEvent(new CustomEvent("connected"));
}
