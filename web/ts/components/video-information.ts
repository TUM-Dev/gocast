import { AlpineComponent } from "./alpine-component";
import { SocketConnections } from "../api/chat-ws";
import { ToggleableElement } from "../utilities/ToggleableElement";
import { RealtimeFacade } from "../utilities/ws";

const CUTOFFLENGTH = 256;
const CUTOFFHEIGHT = 52; // This is the height of one line of level 1 title + one line of plain text

export function videoInformationContext(streamId: number): AlpineComponent {
    // TODO: REST
    const descriptionEl = document.getElementById("description") as HTMLInputElement;
    return {
        viewers: 0 as number,
        description: descriptionEl.innerHTML as string,
        less: descriptionEl.innerHTML.length > CUTOFFLENGTH || descriptionEl.offsetHeight > CUTOFFHEIGHT,

        showFullDescription: new ToggleableElement(),

        init() {
            SocketConnections.ws = new RealtimeFacade("chat/" + streamId);
            Promise.all([this.initWebsocket()]);
        },

        hasDescription(): boolean {
            return this.description.length > 0;
        },

        async initWebsocket() {
            const handler = (data) => {
                if ("viewers" in data) {
                    this.handleViewersUpdate(data);
                } else if ("description" in data) {
                    this.handleDescriptionUpdate(data);
                } else if ("live" in data) {
                    this.handleLiveState(data);
                }
            };
            SocketConnections.ws.subscribe(handler);
        },

        handleViewersUpdate(upd: { viewers: number }) {
            this.viewers = upd.viewers;
        },

        handleDescriptionUpdate(upd: { description: { full: string } }) {
            this.less = upd.description.full.length > CUTOFFLENGTH;
            this.description = upd.description.full;
        },

        handleLiveState(upd: { live: boolean }) {
            if (upd.live) {
                // stream just went live, reload the page to pick up the stream
                window.location.reload();
            }
        },
    } as AlpineComponent;
}

function updateLiveTimeLeft(streamEnd: Date) {
    const now = new Date();
    const timeLeft = streamEnd.getTime() - now.getTime();
    const timeLeftAbs = Math.abs(timeLeft) / 1000;
    document.getElementById("live-time-remaining").innerHTML =
        "Time left: " +
        (timeLeft < 0 ? "-" : "") +
        Math.floor(timeLeftAbs / 60) +
        ":" +
        ("0" + Math.floor(timeLeftAbs % 60)).slice(-2);
}

export function periodicUpdateLiveTimeLeft(streamEnd: string) {
    const streamEndDate = new Date(streamEnd.match("\\d\\d\\d\\d-\\d\\d-\\d\\d \\d\\d:\\d\\d:\\d\\d")[0]);
    setInterval(() => updateLiveTimeLeft(streamEndDate), 1000);
}
