import { AlpineComponent } from "./alpine-component";
import { SocketConnections } from "../api/chat-ws";
import { ToggleableElement } from "../utilities/ToggleableElement";

const CUTOFFLENGTH = 256;

export function videoInformationContext(): AlpineComponent {
    // TODO: REST
    const descriptionEl = document.getElementById("description") as HTMLInputElement;
    return {
        viewers: 0 as number,
        description: descriptionEl.value as string,
        less: descriptionEl.value.length > CUTOFFLENGTH,

        showFullDescription: new ToggleableElement(),

        init() {
            Promise.all([this.initWebsocket()]);
        },

        hasDescription(): boolean {
            return this.description.length > 0;
        },

        async initWebsocket() {
            console.log("init websocket");
            const handler = (data) => {
                if ("viewers" in data) {
                    this.handleViewersUpdate(data);
                } else if ("description" in data) {
                    this.handleDescriptionUpdate(data);
                }
            };
            SocketConnections.ws.addHandler(handler);
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
