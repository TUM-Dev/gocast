import { AlpineComponent } from "./alpine-component";
import { ChatAPI } from "../api/chat";
import { ChatMessage } from "../chat/Chat";
import { WebsocketConnection } from "../chat/ws";

export function chatContext(streamId: number): AlpineComponent {
    return {
        streamId: streamId as number,

        messages: [] as ChatMessage[],

        ws: new WebsocketConnection(`chat/${streamId}`),

        async init() {
            Promise.all([this.loadMessages(), this.initWebsocket()]);
        },

        async initWebsocket() {
            this.ws.subscribe();
        },

        async loadMessages() {
            this.messages = await ChatAPI.getMessages(this.streamId);
        },
    } as AlpineComponent;
}
