import { AlpineComponent } from "./alpine-component";
import { RealtimeFacade } from "../utilities/ws";
import { SocketConnections } from "../api/chat-ws";

export function popupContext(streamId: number): AlpineComponent {
    return {
        init() {
            // subscription?
            SocketConnections.ws = new RealtimeFacade("chat/" + streamId);
            // ws needs to subscribe, so that pop-out chat can work
            const handler = (data) => {};
            SocketConnections.ws.subscribe(handler);
        },
    } as AlpineComponent;
}

export function closeChatOnEscapePressed() {
    document.addEventListener("keyup", function (event) {
        if (event.key === "Escape") {
            window.close();
        }
    });
}
