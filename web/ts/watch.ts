import { scrollChat, shouldScroll, showNewMessageIndicator } from "./chat";
import { NewChatMessage } from "./chat/NewChatMessage";
import { getPlayers } from "./TUMLiveVjs";
import { Get, postData } from "./global";
import { Realtime } from "./socket";
import { copyToClipboard, Time } from "./global";

let currentChatChannel = "";
const retryInt = 5000; //retry connecting to websocket after this timeout

const scrollDelay = 100; // delay before scrolling to bottom to make sure chat is rendered
const pageloaded = new Date();

enum WSMessageType {
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

export { repeatHeatMap } from "./repeat-heatmap";
export { seekbarHighlights } from "./seekbar-highlights";

function sendIDMessage(id: number, type: WSMessageType) {
    return Realtime.get().send(currentChatChannel, {
        payload: {
            type: type,
            id: id,
        },
    });
}

function sendCustomMessage(id: number, type: WSMessageType, optArgs: object = {}) {
    return Realtime.get().send(currentChatChannel, {
        payload: {
            type: type,
            id: id,
            ...optArgs,
        },
    });
}

export const reactToMessage = (id: number, reaction: string) =>
    sendCustomMessage(id, WSMessageType.ReactTo, { reaction });

export const deleteMessage = (id: number) => sendIDMessage(id, WSMessageType.Delete);

export const resolveMessage = (id: number) => sendIDMessage(id, WSMessageType.Resolve);

export const approveMessage = (id: number) => sendIDMessage(id, WSMessageType.Approve);

export const retractMessage = (id: number) => sendIDMessage(id, WSMessageType.Retract);

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

export async function startWebsocket() {
    const streamId = (document.getElementById("streamID") as HTMLInputElement).value;
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
        } else if ("reactions" in data) {
            const event = new CustomEvent("chatreactions", { detail: data });
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
            replyTo: current.reply.id,
            addressedTo: current.addressedTo.map((u) => u.id),
        },
    });
}

export async function fetchMessages(id: number) {
    return await fetch("/api/chat/" + id + "/messages")
        .then((res) => res.json())
        .then((d) => {
            return d;
        });
}

export function startPoll(question: string, pollAnswers: string[]) {
    return Realtime.get().send(currentChatChannel, {
        payload: {
            type: WSMessageType.StartPoll,
            question,
            pollAnswers,
        },
    });
}

export function submitPollOptionVote(pollOptionId: number) {
    return Realtime.get().send(currentChatChannel, {
        payload: {
            type: WSMessageType.SubmitPollOptionVote,
            pollOptionId,
        },
    });
}

export function closeActivePoll() {
    return Realtime.get().send(currentChatChannel, {
        payload: {
            type: WSMessageType.CloseActivePoll,
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

export function contextMenuHandler(e, contextMenu, videoElem) {
    if (contextMenu.shown) return contextMenu;
    e.preventDefault();
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
        const player = getPlayers()[0];
        const vhs = player.tech({ IWillNotUseThisInPlugins: true }).vhs;
        const notAvailable = vhs == null;

        const data = {
            bufferSeconds: notAvailable ? 0 : player.bufferedEnd() - player.currentTime(),
            videoHeight: notAvailable ? 0 : vhs.playlists.media().attributes.RESOLUTION.height,
            videoWidth: notAvailable ? 0 : vhs.playlists.media().attributes.RESOLUTION.width,
            bandwidth: notAvailable ? 0 : vhs.bandwidth,
            mediaRequests: notAvailable ? 0 : vhs.stats.mediaRequests,
            mediaRequestsFailed: notAvailable ? 0 : vhs.stats.mediaRequestsErrored,
        };
        const event = new CustomEvent("newvideostats", { detail: data });
        window.dispatchEvent(event);
    },
    clear() {
        if (this.videoStatIntervalId != null) {
            clearInterval(this.videoStatIntervalId);
            this.videoStatIntervalId = null;
        }
    },
};

export function onShift(e) {
    if (document.activeElement.tagName !== "INPUT" && document.activeElement.tagName !== "TEXTAREA") {
        switch (e.key) {
            case "?": {
                toggleShortcutsModal();
            }
        }
    }
}

export function toggleShortcutsModal() {
    const el = document.getElementById("shortcuts-help-modal");
    if (el !== undefined) {
        if (el.classList.contains("hidden")) {
            el.classList.remove("hidden");
        } else {
            el.classList.add("hidden");
        }
    }
}

export class ShareURL {
    url: string;
    includeTimestamp: boolean;
    timestamp: string;

    copied: boolean; // success indicator

    private baseUrl: string;
    private playerHasTime: Promise<boolean>;
    private timestampArgument: string;

    constructor() {
        this.baseUrl = [location.protocol, "//", location.host, location.pathname].join(""); // get rid of query
        this.url = this.baseUrl;
        this.includeTimestamp = false;
        this.copied = false;

        const player = getPlayers()[0];
        player.ready(() => {
            player.on("loadedmetadata", () => {
                this.playerHasTime = Promise.resolve(true);
            });
        });
    }

    async setURL(shouldFetchPlayerTime?: boolean) {
        if (this.includeTimestamp) {
            if (shouldFetchPlayerTime || !this.timestamp) {
                const player = getPlayers()[0];
                await this.playerHasTime;
                this.timestamp = Time.FromSeconds(player.currentTime()).toStringWithLeadingZeros();
                await this.updateURLStateFromTimestamp();
            } else {
                await this.updateURLStateFromTimestamp();
            }
            this.url = this.baseUrl + this.timestampArgument;
        } else {
            this.url = this.baseUrl;
        }
    }

    copyURL() {
        copyToClipboard(this.url);
        this.copied = true;
        setTimeout(() => (this.copied = false), 1000);
    }

    private async updateURLStateFromTimestamp() {
        const trim = this.timestamp.substring(0, 9);
        const split = trim.split(":");
        if (split.length != 3) {
            this.url = this.baseUrl;
        } else {
            const h = +split[0];
            const m = +split[1];
            const s = +split[2];
            if (isNaN(h) || isNaN(m) || isNaN(s) || h > 60 || m > 60 || s > 60 || h < 0 || m < 0 || s < 0) {
                this.url = this.baseUrl;
            } else {
                const inSeconds = s + 60 * m + 60 * 60 * h;
                this.timestampArgument = `?t=${inSeconds}`;
            }
        }
    }
}
