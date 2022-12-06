import { scrollChat, shouldScroll, showNewMessageIndicator } from "./chat";
import { NewChatMessage } from "./chat/NewChatMessage";
import { Realtime } from "./socket";

let currentChatChannel = "";

const scrollDelay = 100; // delay before scrolling to bottom to make sure chat is rendered

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

function sendActionMessage(type: WSMessageType, payload: object = {}) {
    payload["type"] = type;
    return Realtime.get().send(currentChatChannel, { payload });
}

export const likeMessage = (id: number) => sendActionMessage(WSMessageType.Like, { id });

export const deleteMessage = (id: number) => sendActionMessage(WSMessageType.Delete, { id });

export const resolveMessage = (id: number) => sendActionMessage(WSMessageType.Resolve, { id });

export const approveMessage = (id: number) => sendActionMessage(WSMessageType.Approve, { id });

export const retractMessage = (id: number) => sendActionMessage(WSMessageType.Retract, { id });

export const closeActivePoll = () => sendActionMessage(WSMessageType.CloseActivePoll);

export const submitPollOptionVote = (pollOptionId: number) =>
    sendActionMessage(WSMessageType.SubmitPollOptionVote, { pollOptionId });

export const startPoll = (question: string, pollAnswers: string[]) =>
    sendActionMessage(WSMessageType.StartPoll, { question, pollAnswers });

export function initChatScrollListener() {
    const chatBox = document.getElementById("chatBox") as HTMLDivElement;
    if (!chatBox) {
        return;
    }
    chatBox.addEventListener("scroll", function (e) {
        if (chatBox.scrollHeight - chatBox.scrollTop === chatBox.offsetHeight) {
            window.dispatchEvent(new CustomEvent("messageindicator", { detail: { show: false } }));
        }
    });
}

export async function startWebsocket(streamId: number) {
    currentChatChannel = `chat/${streamId}`;

    const messageHandler = function (data) {
        if ("viewers" in data) {
            window.dispatchEvent(new CustomEvent("viewers", { detail: { viewers: data["viewers"] } }));
        } else if ("live" in data) {
            if (data["live"]) {
                // stream start, refresh page
                window.location.reload();
            } else {
                // stream end, show message
                window.dispatchEvent(new CustomEvent("streamended"));
            }
        } else if ("server" in data) {
            const scroll = shouldScroll();
            const serverElem = createServerMessage(data);
            document.getElementById("chatBox").appendChild(serverElem);
            if (scroll) {
                setTimeout(scrollChat, scrollDelay);
            } else {
                showNewMessageIndicator();
            }
        } else if ("message" in data) {
            data["replies"] = []; // go serializes this empty list as `null`
            // reply
            if (data["replyTo"].Valid) {
                // reply
                const event = new CustomEvent("chatreply", { detail: data });
                window.dispatchEvent(event);
            } else {
                // message
                const scroll = shouldScroll();
                const event = new CustomEvent("chatmessage", { detail: data });
                window.dispatchEvent(event);
                if (scroll) {
                    setTimeout(scrollChat, scrollDelay);
                } else {
                    showNewMessageIndicator();
                }
            }
        } else if ("pollOptions" in data) {
            const event = new CustomEvent("chatnewpoll", { detail: data });
            window.dispatchEvent(event);
        } else if ("pollOptionId" in data) {
            const event = new CustomEvent("polloptionvotesupdate", { detail: data });
            window.dispatchEvent(event);
        } else if ("pollOptionResults" in data) {
            const event = new CustomEvent("polloptionresult", { detail: data });
            window.dispatchEvent(event);
        } else if ("likes" in data) {
            const event = new CustomEvent("chatlike", { detail: data });
            window.dispatchEvent(event);
        } else if ("delete" in data) {
            const event = new CustomEvent("chatdelete", { detail: data });
            window.dispatchEvent(event);
        } else if ("resolve" in data) {
            const event = new CustomEvent("chatresolve", { detail: data });
            window.dispatchEvent(event);
        } else if ("approve" in data) {
            const event = new CustomEvent("chatapprove", { detail: data });
            window.dispatchEvent(event);
        } else if ("retract" in data) {
            const event = new CustomEvent("chatretract", { detail: data });
            window.dispatchEvent(event);
        } else if ("title" in data) {
            const event = new CustomEvent("titleupdate", { detail: data });
            window.dispatchEvent(event);
        } else if ("description" in data) {
            const event = new CustomEvent("descriptionupdate", { detail: data });
            window.dispatchEvent(event);
        }
    };

    // TODO: check if connected and update
    //window.dispatchEvent(new CustomEvent("connected"));
    //window.dispatchEvent(new CustomEvent("disconnected"));

    await Realtime.get().subscribeChannel(currentChatChannel, messageHandler);
    window.dispatchEvent(new CustomEvent("connected"));
}

export function createServerMessage(msg) {
    const serverElem = document.createElement("div");
    switch (msg["type"]) {
        case "error":
            serverElem.classList.add("text-danger", "font-semibold");
            break;
        case "info":
            serverElem.classList.add("text-4");
            break;
        case "warn":
            serverElem.classList.add("text-warn", "font-semibold");
            break;
    }
    serverElem.classList.add("text-sm", "p-2");
    serverElem.innerText = msg["server"];
    return serverElem;
}

export function sendMessage(current: NewChatMessage) {
    return Realtime.get().send(currentChatChannel, {
        payload: {
            type: WSMessageType.Message,
            msg: current.message,
            anonymous: current.anonymous,
            replyTo: current.replyTo,
            addressedTo: current.addressedTo.map((u) => u.id),
        },
    });
}

export function getPollOptionWidth(pollOptions, pollOption) {
    const minWidth = 1;
    const maxWidth = 100;
    const maxVotes = Math.max(...pollOptions.map(({ votes: v }) => v));

    if (pollOption.votes == 0) return `${minWidth.toString()}%`;

    const fractionOfMax = pollOption.votes / maxVotes;
    const fractionWidth = minWidth + fractionOfMax * (maxWidth - minWidth);
    return `${Math.ceil(fractionWidth).toString()}%`;
}
