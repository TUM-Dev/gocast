import { AlpineComponent } from "./alpine-component";
import { RealtimeFacade } from "../utilities/ws";
import { SocketConnections } from "../api/chat-ws";

export function popupContext(streamId: number): AlpineComponent {
    return {
        init() {
            SocketConnections.ws = new RealtimeFacade("chat/" + streamId);
        },
    } as AlpineComponent;
}
