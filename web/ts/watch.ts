import { scrollChat, shouldScroll, showNewMessageIndicator } from "./chat";
import { NewChatMessage } from "./chat/NewChatMessage";
import { getPlayer } from "./TUMLiveVjs";
import { Get, postData } from "./global";
import { Realtime } from "./socket";
import { copyToClipboard } from "./global";

let currentChatChannel = "";
const retryInt = 5000; //retry connecting to websocket after this timeout

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
    Retract = "retract",
    Resolve = "resolve",
}

function sendIDMessage(id: number, type: WSMessageType) {
    return Realtime.get().send(currentChatChannel, {
        payload: {
            type: type,
            id: id,
        },
    });
}

export const likeMessage = (id: number) => sendIDMessage(id, WSMessageType.Like);

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

export function contextMenuHandler(e, contextMenu) {
    if (contextMenu.shown) return contextMenu;
    e.preventDefault();
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
        const player = getPlayer();
        const vhs = player.tech({ IWillNotUseThisInPlugins: true }).vhs;
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
    },
    clear() {
        if (this.videoStatIntervalId != null) {
            clearInterval(this.videoStatIntervalId);
            this.videoStatIntervalId = null;
        }
    },
};

/*const heatMapExamplePath =
    "M 0.0,100.0 C 1.0,89.5 2.0,55.4 5.0,47.3 C 8.0,39.2 11.0,55.7 15.0,59.4 C 19.0,63.0 21.0,63.1 25.0,65.6 C 29.0,68.0 31.0,69.6 35.0,71.6 C 39.0,73.5 41.0,73.8 45.0,75.3 C 49.0,76.9 51.0,77.7 55.0,79.4 C 59.0,81.0 61.0,82.1 65.0,83.6 C 69.0,85.1 71.0,85.8 75.0,86.8 C 79.0,87.8 81.0,88.0 85.0,88.5 C 89.0,89.0 91.0,89.1 95.0,89.3 C 99.0,89.4 101.0,89.1 105.0,89.2 C 109.0,89.3 111.0,89.5 115.0,89.7 C 119.0,89.8 121.0,89.9 125.0,90.0 C 129.0,90.1 131.0,90.0 135.0,90.0 C 139.0,90.0 141.0,90.0 145.0,90.0 C 149.0,90.0 151.0,90.0 155.0,90.0 C 159.0,90.0 161.0,90.0 165.0,90.0 C 169.0,90.0 171.0,90.0 175.0,90.0 C 179.0,90.0 181.0,90.0 185.0,90.0 C 189.0,90.0 191.0,90.0 195.0,90.0 C 199.0,90.0 201.0,90.0 205.0,90.0 C 209.0,90.0 211.0,90.0 215.0,90.0 C 219.0,90.0 221.0,90.0 225.0,90.0 C 229.0,90.0 231.0,90.0 235.0,90.0 C 239.0,90.0 241.0,90.0 245.0,90.0 C 249.0,90.0 251.0,90.0 255.0,90.0 C 259.0,90.0 261.0,90.0 265.0,90.0 C 269.0,90.0 271.0,90.0 275.0,90.0 C 279.0,90.0 281.0,90.0 285.0,90.0 C 289.0,90.0 291.0,90.0 295.0,90.0 C 299.0,90.0 301.0,90.0 305.0,90.0 C 309.0,90.0 311.0,90.0 315.0,90.0 C 319.0,90.0 321.0,90.0 325.0,90.0 C 329.0,90.0 331.0,90.0 335.0,90.0 C 339.0,90.0 341.0,90.0 345.0,90.0 C 349.0,90.0 351.0,90.0 355.0,90.0 C 359.0,90.0 361.0,90.0 365.0,90.0 C 369.0,90.0 371.0,90.0 375.0,90.0 C 379.0,90.0 381.0,90.0 385.0,90.0 C 389.0,90.0 391.0,90.0 395.0,90.0 C 399.0,90.0 401.0,90.0 405.0,90.0 C 409.0,90.0 411.0,90.0 415.0,90.0 C 419.0,90.0 421.0,90.0 425.0,90.0 C 429.0,90.0 431.0,90.0 435.0,90.0 C 439.0,90.0 441.0,90.0 445.0,90.0 C 449.0,90.0 451.0,90.5 455.0,90.0 C 459.0,89.5 461.0,88.4 465.0,87.7 C 469.0,87.0 471.0,87.0 475.0,86.7 C 479.0,86.3 481.0,85.8 485.0,85.9 C 489.0,86.0 491.0,86.7 495.0,87.0 C 499.0,87.4 501.0,87.4 505.0,87.6 C 509.0,87.8 511.0,88.1 515.0,88.1 C 519.0,88.1 521.0,87.9 525.0,87.6 C 529.0,87.4 531.0,87.6 535.0,86.7 C 539.0,85.8 541.0,84.9 545.0,83.3 C 549.0,81.6 551.0,81.2 555.0,78.3 C 559.0,75.5 561.0,73.0 565.0,69.0 C 569.0,65.0 571.0,62.1 575.0,58.4 C 579.0,54.6 581.0,54.5 585.0,50.1 C 589.0,45.6 591.0,42.4 595.0,36.1 C 599.0,29.7 601.0,25.1 605.0,18.4 C 609.0,11.7 611.0,6.3 615.0,2.6 C 619.0,-1.0 621.0,-1.4 625.0,0.0 C 629.0,1.4 631.0,5.6 635.0,9.6 C 639.0,13.7 641.0,16.8 645.0,20.2 C 649.0,23.6 651.0,25.2 655.0,26.6 C 659.0,28.1 661.0,26.7 665.0,27.3 C 669.0,27.8 671.0,28.3 675.0,29.5 C 679.0,30.6 681.0,31.1 685.0,32.8 C 689.0,34.5 691.0,36.2 695.0,37.9 C 699.0,39.6 701.0,39.3 705.0,41.5 C 709.0,43.6 711.0,46.0 715.0,48.6 C 719.0,51.3 721.0,52.5 725.0,54.7 C 729.0,56.9 731.0,57.7 735.0,59.8 C 739.0,61.8 741.0,63.8 745.0,65.0 C 749.0,66.2 751.0,65.4 755.0,65.7 C 759.0,66.0 761.0,66.3 765.0,66.5 C 769.0,66.7 771.0,66.8 775.0,66.6 C 779.0,66.4 781.0,66.2 785.0,65.6 C 789.0,65.0 791.0,63.7 795.0,63.5 C 799.0,63.3 801.0,64.7 805.0,64.6 C 809.0,64.6 811.0,63.5 815.0,63.1 C 819.0,62.6 821.0,62.0 825.0,62.3 C 829.0,62.6 831.0,64.1 835.0,64.7 C 839.0,65.3 841.0,65.2 845.0,65.5 C 849.0,65.8 851.0,66.0 855.0,66.1 C 859.0,66.2 861.0,66.0 865.0,66.1 C 869.0,66.2 871.0,66.4 875.0,66.8 C 879.0,67.1 881.0,67.0 885.0,67.7 C 889.0,68.4 891.0,69.6 895.0,70.2 C 899.0,70.9 901.0,70.4 905.0,70.9 C 909.0,71.4 911.0,71.9 915.0,72.8 C 919.0,73.8 921.0,74.8 925.0,75.8 C 929.0,76.7 931.0,77.2 935.0,77.7 C 939.0,78.3 941.0,78.5 945.0,78.7 C 949.0,78.8 951.0,78.8 955.0,78.6 C 959.0,78.4 961.0,77.8 965.0,77.8 C 969.0,77.7 971.0,78.3 975.0,78.3 C 979.0,78.4 981.0,78.5 985.0,78.0 C 989.0,77.6 992.0,76.5 995.0,76.1 C 998.0,75.7 999.0,71.3 1000.0,76.1 C 1001.0,80.9 1000.0,95.2 1000.0,100.0";
*/

const repeatMapScale = 90;

export const repeatHeatMap = {
    seekBar: null,

    init(streamID: number) {
        this.streamID = streamID;
        setTimeout(() => this.updateHeatMap(), 0);

        const player = getPlayer();
        player.ready(() => {
            this.seekBar = document.querySelector(".vjs-progress-control");
            this.injectElementIntoVjs();

            new ResizeObserver(this.updateSize.bind(this)).observe(this.seekBar);
            this.updateSize();
        });
    },

    injectElementIntoVjs() {
        const heatmap = document.querySelector(".heatmap-wrap");
        this.seekBar.append(heatmap);
    },

    updateSize() {
        const event = new CustomEvent("updateheatmapsize", {
            detail: this.getSeekbarInfo(),
        });
        window.dispatchEvent(event);
    },

    valuesToArray(listOfChunkValues) {
        const max = listOfChunkValues.reduce((res, item) => (res > item.value ? res : item.value), 0);
        return [...Array(100)].map((val, index) => {
            const item = listOfChunkValues.find((e) => e.index === index);
            const size = item ? (repeatMapScale / max) * item.value : 0;
            return repeatMapScale - size;
        });
    },

    updateHeatMap() {
        const values = JSON.parse(Get(`/api/seekReport/${this.streamID}`)).values;
        const event = new CustomEvent("updateheatmappath", {
            detail: { heatMapPath: this.genHeatMapPath(this.valuesToArray(values)) },
        });
        window.dispatchEvent(event);
    },

    genHeatMapPath(values) {
        let res = "M 0.0,100.0";

        for (let i = 0; i < 1000; i += 10) {
            const currX = i + 1;
            const lastVal = values[i / 10 - 1] ?? 0;
            const currVal = values[i / 10];

            const slope = (lastVal + currVal) / 2;
            const arcHandleA = `${currX.toFixed(1)},${lastVal.toFixed(1)}`;
            const arcHandleB = `${(currX + 2).toFixed(1)},${slope.toFixed(1)}`;
            const to = `${(currX + 6).toFixed(1)},${currVal.toFixed(1)}`;

            const p = ` C ${arcHandleA} ${arcHandleB} ${to}`;
            res += p;
        }

        const lastVal = values[values.length - 1];
        const slope = (lastVal + 100) / 2;

        res += ` C 1001.0,${lastVal} 1000.0,${slope} 1000.0,100.0`;

        return res;
    },

    data() {
        return "";
    },

    getSeekbarInfo() {
        const seekBar = document.querySelector(".vjs-progress-holder");
        if (!seekBar) {
            return { x: "0px", width: "0px" };
        }

        const marginLeft = window.getComputedStyle(seekBar).marginLeft;
        const width = seekBar.getBoundingClientRect().width;
        return {
            x: marginLeft,
            width: width + "px",
        };
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
    private baseUrl: string;

    url: string;
    includeTimestamp: boolean;
    timestamp: string;
    openTime: number;

    copied: boolean; // success indicator

    constructor() {
        this.baseUrl = [location.protocol, "//", location.host, location.pathname].join(""); // get rid of query
        this.url = this.baseUrl;
        this.includeTimestamp = false;
        this.copied = false;

        const player = getPlayer();
        player.ready(() => {
            player.on("loadedmetadata", () => {
                this.openTime = player.currentTime();
            });
        });
    }

    copyURL() {
        copyToClipboard(this.url);
        this.copied = true;
        setTimeout(() => (this.copied = false), 3000);
    }

    setURL() {
        if (this.includeTimestamp) {
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
                    this.url = `${this.baseUrl}?t=${inSeconds}`;
                }
            }
        } else {
            this.url = this.baseUrl;
        }
    }

    setTimestamp() {
        const d = new Date(this.openTime * 1000);
        const h = ShareURL.padZero(d.getUTCHours());
        const m = ShareURL.padZero(d.getUTCMinutes());
        const s = ShareURL.padZero(d.getSeconds());
        this.timestamp = `${h}:${m}:${s}`;
    }

    private static padZero(i) {
        if (i < 10) {
            i = "0" + i;
        }
        return i;
    }
}
