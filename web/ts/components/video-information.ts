import { AlpineComponent } from "./alpine-component";
import { SocketConnections } from "../api/chat-ws";

export function videoInformationContext(): AlpineComponent {
    return {
        viewers: 0 as number,

        init() {
            Promise.all([this.initWebsocket()]);
        },

        async initWebsocket() {
            console.log("init websocket");
            const handler = (data) => {
                if ("viewers" in data) {
                    this.handleViewersUpdate(data);
                } else if ("description" in data) {
                    console.log(data);
                }
            };
            SocketConnections.ws.addHandler(handler);
        },

        handleViewersUpdate(upd: { viewers: number }) {
            this.viewers = upd.viewers;
        },
    } as AlpineComponent;
}
