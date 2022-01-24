class Watch {
    private chatInput: HTMLInputElement;

    constructor() {
        if (document.getElementById("chatForm") != null) {
            const appHeight = () => {
                const doc = document.documentElement;
                doc.style.setProperty("--chat-height", `calc(${window.innerHeight}px - 5rem)`);
            };
            window.addEventListener("resize", appHeight);
            appHeight();
            this.chatInput = document.getElementById("chatInput") as HTMLInputElement;
        }
    }
}

let ws: WebSocket;
let retryInt = 5000; //retry connecting to websocket after this timeout
const pageloaded = new Date();

function initChatScrollListener() {
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

function scrollChatIfNeeded() {
    const c = document.getElementById("chatBox");
    // 150px grace offset to avoid showing message when close to bottom
    if (c.scrollHeight - c.scrollTop <= c.offsetHeight + 150) {
        c.scrollTop = c.scrollHeight;
    } else {
        window.dispatchEvent(new CustomEvent("messageindicator", { detail: { show: true } }));
    }
}

function scrollToLatestMessage() {
    const c = document.getElementById("chatBox");
    c.scrollTo({ top: c.scrollHeight, behavior: "smooth" });
    window.dispatchEvent(new CustomEvent("messageindicator", { detail: { show: false } }));
}

function startWebsocket() {
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
            const serverElem = createServerMessage(data);
            document.getElementById("chatBox").appendChild(serverElem);
            scrollChatIfNeeded();
        } else if ("message" in data) {
            data["replies"] = []; // go serializes this empty list as `null`
            // reply
            if (data["replyTo"].Valid) {
                // reply
                const event = new CustomEvent("chatreply", { detail: data });
                window.dispatchEvent(event);
            } else {
                // message
                const event = new CustomEvent("chatmessage", { detail: data });
                window.dispatchEvent(event);
                scrollChatIfNeeded();
            }
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

startWebsocket();

function createServerMessage(msg) {
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

function sendMessage(message: string, anonymous: boolean, replyTo: number) {
    ws.send(
        JSON.stringify({
            msg: message,
            anonymous: anonymous,
            replyTo: replyTo,
        }),
    );
}

function showDisconnectedMsg() {
    if (document.getElementById("disconnectMsg") !== null) {
        document.getElementById("disconnectMsg").classList.remove("hidden");
    }
}

function hideDisconnectedMsg() {
    if (document.getElementById("disconnectMsg") !== null) {
        document.getElementById("disconnectMsg").classList.add("hidden");
    }
}

new Watch();
