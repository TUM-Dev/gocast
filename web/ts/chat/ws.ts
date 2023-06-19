import { MessageHandlerFn, Realtime } from "../socket";

export abstract class WebsocketConnection {
    private readonly channel: string;

    connected: boolean;

    constructor(channel: string) {
        this.channel = channel;
    }

    async subscribe(handler: MessageHandlerFn) {
        Realtime.get()
            .subscribeChannel(this.channel, handler)
            .then(() => (this.connected = true));
    }

    protected send(payload: object = {}) {
        return Realtime.get().send(this.channel, payload);
    }
}
