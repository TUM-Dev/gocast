let chatInput;

export class Watch {
    constructor() {
        if (document.getElementById("chatForm") != null) {
            const appHeight = () => {
                const doc = document.documentElement;
                doc.style.setProperty("--chat-height", `calc(${window.innerHeight}px - 5rem)`);
            };
            window.addEventListener("resize", appHeight);
            appHeight();
            chatInput = document.getElementById("chatInput") as HTMLInputElement;
        }
    }
}

let ws: WebSocket;
let retryInt = 5000; //retry connecting to websocket after this timeout
let orderByLikes = false; // sorting by likes or by time

const pageloaded = new Date();

export function likeMessage(id: number) {
    ws.send(
        JSON.stringify({
            type: "like",
            id: id,
        }),
    );
}

export function sortMessages(messages) {
    messages.sort((m1, m2) => {
        if (orderByLikes) {
            if (m1.likes === m2.likes) {
                return m2.id - m1.id; // same amount of likes -> newer messages up
            }
            return m2.likes - m1.likes; // more likes -> up
        } else {
            return m1.ID < m2.ID ? -1 : 1; // newest messages last
        }
    });
}

export function setOrder(obl: boolean) {
    orderByLikes = obl;
}

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

export function shouldScroll(): boolean {
    if (orderByLikes) {
        return false; // only scroll if sorting by time
    }
    const c = document.getElementById("chatBox");
    return c.scrollHeight - c.scrollTop <= c.offsetHeight;
}

function showNewMessageIndicator() {
    window.dispatchEvent(new CustomEvent("messageindicator", { detail: { show: true } }));
}

export function scrollChat() {
    console.log("scrollChatIfNeeded");
    if (orderByLikes) {
        return; // only scroll if sorting by time
    }
    const c = document.getElementById("chatBox");
    c.scrollTop = c.scrollHeight;
}

export function scrollToLatestMessage() {
    const c = document.getElementById("chatBox");
    c.scrollTo({ top: c.scrollHeight, behavior: "smooth" });
    window.dispatchEvent(new CustomEvent("messageindicator", { detail: { show: false } }));
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
        console.log("got message");
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
            console.log("scroll: ", scroll);
            const serverElem = createServerMessage(data);
            document.getElementById("chatBox").appendChild(serverElem);
            if (scroll) {
                setTimeout(scrollChat, 100); // wait a bit to make sure the message is added before scrolling
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
                    setTimeout(scrollChat, 100); // wait a bit to make sure the message is added before scrolling
                } else {
                    showNewMessageIndicator();
                }
            }
        } else if ("likes" in data) {
            const event = new CustomEvent("chatlike", { detail: data });
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
            type: "message",
            msg: message,
            anonymous: anonymous,
            replyTo: replyTo,
        }),
    );
}

export function showDisconnectedMsg() {
    if (document.getElementById("disconnectMsg") !== null) {
        document.getElementById("disconnectMsg").classList.remove("hidden");
    }
}

export function hideDisconnectedMsg() {
    if (document.getElementById("disconnectMsg") !== null) {
        document.getElementById("disconnectMsg").classList.add("hidden");
    }
}
