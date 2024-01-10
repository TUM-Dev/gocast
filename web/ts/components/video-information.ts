import { AlpineComponent } from "./alpine-component";
import { SocketConnections } from "../api/chat-ws";
import { ToggleableElement } from "../utilities/ToggleableElement";
import { RealtimeFacade } from "../utilities/ws";

const CUTOFFLENGTH = 256;
const CUTOFFHEIGHT = 56; // This is the height of one line of level 1 title + one line of plain text

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
            console.log(descriptionEl.offsetHeight);
            console.log(descriptionEl.getBoundingClientRect().height);
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
    } as AlpineComponent;
}
