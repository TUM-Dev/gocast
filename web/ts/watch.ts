import { hideDisconnectedMsg, scrollChat, shouldScroll, showDisconnectedMsg, showNewMessageIndicator } from "./chat";

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
        hideDisconnectedMsg();
    };

    ws.onmessage = function (m) {
        const data = JSON.parse(m.data);
        if ("viewers" in data && document.getElementById("viewerCount") != null) {
            document.getElementById("viewerCount").innerText = data["viewers"];
        } else if ("live" in data) {
            window.location.reload();
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
        } else if ("likes" in data) {
            const event = new CustomEvent("chatlike", { detail: data });
            window.dispatchEvent(event);
        } else if ("delete" in data) {
            const event = new CustomEvent("chatdelete", { detail: data });
            window.dispatchEvent(event);
        } else if ("resolve" in data) {
            const event = new CustomEvent("chatresolve", { detail: data });
            window.dispatchEvent(event);
        }
    };

    ws.onclose = function () {
        // connection closed, discard old websocket and create a new one after backoff
        // don't recreate new connection if page has been loaded more than 12 hours ago
        if (new Date().valueOf() - pageloaded.valueOf() > 1000 * 60 * 60 * 12) {
            return;
        }
        showDisconnectedMsg();
        ws = null;
        retryInt *= 2; // exponential backoff
        setTimeout(startWebsocket, retryInt);
    };

    ws.onerror = function (err) {
        showDisconnectedMsg();
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

export function sendMessage(message: string, anonymous: boolean, replyTo: number) {
    ws.send(
        JSON.stringify({
            type: WSMessageType.Message,
            msg: message,
            anonymous: anonymous,
            replyTo: replyTo,
        }),
    );
}
