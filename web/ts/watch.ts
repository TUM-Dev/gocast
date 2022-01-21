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
const pageloaded = new Date();

export function startWebsocket() {
    const wsProto = window.location.protocol === "https:" ? `wss://` : `ws://`;
    const streamid = (document.getElementById("streamID") as HTMLInputElement).value;
    ws = new WebSocket(`${wsProto}${window.location.host}/api/chat/${streamid}/ws`);
    const cf = document.getElementById("chatForm");
    if (cf !== null && cf != undefined) {
        (document.getElementById("chatForm") as HTMLFormElement).addEventListener("submit", (e) => submitChat(e));
    }
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
            document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight;
        } else if ("msg" in data) {
            const chatElem = createMessageElement(data);
            document.getElementById("chatBox").appendChild(chatElem);
            document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight;
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

/*
    while I'm not a fan of huge frontend frameworks, this is a good example why they can be useful.
     */
export function createMessageElement(m): HTMLDivElement {
    // Header:
    const chatElem = document.createElement("div") as HTMLDivElement;
    chatElem.classList.add("rounded", "py-2");
    const chatHeader = document.createElement("div") as HTMLDivElement;
    chatHeader.classList.add("flex", "flex-row");
    const chatNameField = document.createElement("p") as HTMLParagraphElement;
    chatNameField.classList.add("text-sm", "grow", "font-semibold");
    if (m["admin"]) {
        chatNameField.classList.add("text-warn");
    }
    chatNameField.innerText = m["name"];
    chatHeader.appendChild(chatNameField);

    const d = new Date();
    d.setTime(Date.now());
    const chatTimeField = document.createElement("p") as HTMLParagraphElement;
    chatTimeField.classList.add("text-4", "text-xs");
    chatTimeField.innerText = ("0" + d.getHours()).slice(-2) + ":" + ("0" + d.getMinutes()).slice(-2);
    chatHeader.appendChild(chatTimeField);
    chatElem.appendChild(chatHeader);

    // Message:
    const chatMessage = document.createElement("p") as HTMLParagraphElement;
    chatMessage.classList.add("text-3", "break-words");
    chatMessage.innerText = m["msg"];
    chatElem.appendChild(chatMessage);
    return chatElem;
}

export function submitChat(e: Event) {
    e.preventDefault();

    const anonCheckbox: HTMLInputElement = document.getElementById("anonymous") as HTMLInputElement;
    ws.send(
        JSON.stringify({
            msg: chatInput.value,
            anonymous: anonCheckbox ? anonCheckbox.checked : false,
        }),
    );

    chatInput.value = "";
    return false; //prevent form submission
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
