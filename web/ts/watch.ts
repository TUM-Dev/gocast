import { scrollChat, shouldScroll, showNewMessageIndicator } from "./chat";
import { NewChatMessage } from "./chat/NewChatMessage";
import { getPlayers } from "./TUMLiveVjs";
import videojs from "video.js";

let chatInput: HTMLInputElement;

export class Watch {
    constructor() {
        // Empty
    }
}

let ws: WebSocket;
let retryInt = 5000; //retry connecting to websocket after this timeout

const scrollDelay = 100; // delay before scrolling to bottom to make sure chat is rendered
const pageloaded = new Date();

enum WSMessageType {
    Message = "message",
    Like = "like",
    Delete = "delete",
    StartPoll = "start_poll",
    SubmitPollOptionVote = "submit_poll_option_vote",
    CloseActivePoll = "close_active_poll",
    Approve = "approve",
    Resolve = "resolve",
}

function sendIDMessage(id: number, type: WSMessageType) {
    ws.send(
        JSON.stringify({
            type: type,
            id: id,
        }),
    );
}

export const likeMessage = (id: number) => sendIDMessage(id, WSMessageType.Like);

export const deleteMessage = (id: number) => sendIDMessage(id, WSMessageType.Delete);

export const resolveMessage = (id: number) => sendIDMessage(id, WSMessageType.Resolve);

export const approveMessage = (id: number) => sendIDMessage(id, WSMessageType.Approve);

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

export function startWebsocket() {
    const wsProto = window.location.protocol === "https:" ? `wss://` : `ws://`;
    const streamid = (document.getElementById("streamID") as HTMLInputElement).value;
    ws = new WebSocket(`${wsProto}${window.location.host}/api/chat/${streamid}/ws`);
    initChatScrollListener();
    ws.onopen = function (e) {
        window.dispatchEvent(new CustomEvent("connected"));
    };

    ws.onmessage = function (m) {
        const data = JSON.parse(m.data);
        if ("viewers" in data && document.getElementById("viewerCount") != null) {
            document.getElementById("viewerCount").innerText = data["viewers"];
        } else if ("live" in data) {
            if (data["live"]) {
                // stream start, refresh page
                window.location.reload();
            } else {
                // stream end, show message
                window.dispatchEvent(new CustomEvent("streamended"));
            }
        } else if ("paused" in data) {
            const paused: boolean = data["paused"];
            if (paused) {
                //window.dispatchEvent(new CustomEvent("pausestart"))
            } else {
                window.dispatchEvent(new CustomEvent("pauseend"));
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
        } else if ("title" in data) {
            const event = new CustomEvent("titleupdate", { detail: data });
            window.dispatchEvent(event);
        } else if ("description" in data) {
            const event = new CustomEvent("descriptionupdate", { detail: data });
            window.dispatchEvent(event);
        }
    };

    ws.onclose = function () {
        // connection closed, discard old websocket and create a new one after backoff
        // don't recreate new connection if page has been loaded more than 12 hours ago
        if (new Date().valueOf() - pageloaded.valueOf() > 1000 * 60 * 60 * 12) {
            return;
        }
        window.dispatchEvent(new CustomEvent("disconnected"));
        ws = null;
        retryInt *= 2; // exponential backoff
        setTimeout(startWebsocket, retryInt);
    };

    ws.onerror = function (err) {
        window.dispatchEvent(new CustomEvent("disconnected"));
    };
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
    ws.send(
        JSON.stringify({
            type: WSMessageType.Message,
            msg: current.message,
            anonymous: current.anonymous,
            replyTo: current.replyTo,
            addressedTo: current.addressedTo.map((u) => u.id),
        }),
    );
}

export async function fetchMessages(id: number) {
    return await fetch("/api/chat/" + id + "/messages")
        .then((res) => res.json())
        .then((d) => {
            return d;
        });
}

export function startPoll(question: string, pollAnswers: string[]) {
    ws.send(
        JSON.stringify({
            type: WSMessageType.StartPoll,
            question,
            pollAnswers,
        }),
    );
}

export function submitPollOptionVote(pollOptionId: number) {
    ws.send(
        JSON.stringify({
            type: WSMessageType.SubmitPollOptionVote,
            pollOptionId,
        }),
    );
}

export function closeActivePoll() {
    ws.send(
        JSON.stringify({
            type: WSMessageType.CloseActivePoll,
        }),
    );
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

export function contextMenuHandler(e, contextMenu) {
    if (contextMenu.shown) return contextMenu;
    e.preventDefault();
    //TODO single video element here, may cause problems
    const videoElem = document.querySelector("#my-video");
    return {
        shown: true,
        locX: e.clientX - videoElem.getBoundingClientRect().left,
        locY: e.clientY - videoElem.getBoundingClientRect().top,
    };
}

export const videoStatListener = {
    videoStatIntervalId: null,
    listen() {
        if (this.videoStatIntervalId != null) {
            return;
        }
        this.videoStatIntervalId = setInterval(this.update, 1000);
        this.update();
    },
    update() {
        for (const player of getPlayers()) {
            const vhs = player.tech().vhs;
            const notAvailable = vhs == null;

            const data = {
                bufferSeconds: notAvailable ? 0 : player.bufferedEnd() - player.currentTime(),
                videoHeight: notAvailable ? 0 : vhs.playlists.media().attributes.RESOLUTION.height,
                videoWidth: notAvailable ? 0 : vhs.playlists.media().attributes.RESOLUTION.width,
                bandwidth: notAvailable ? 0 : vhs.bandwidth, //player.tech().vhs.bandwidth(),
                mediaRequests: notAvailable ? 0 : vhs.stats.mediaRequests,
                mediaRequestsFailed: notAvailable ? 0 : vhs.stats.mediaRequestsErrored,
            };
            const event = new CustomEvent("newvideostats", { detail: data });
            window.dispatchEvent(event);
        }
    },
    clear() {
        if (this.videoStatIntervalId != null) {
            clearInterval(this.videoStatIntervalId);
            this.videoStatIntervalId = null;
        }
    },
};
