import { MessageHandlerFn, Realtime } from "../socket";

export class RealtimeFacade {
    private readonly channel: string;

    constructor(channel: string) {
        this.channel = channel;
    }

    async subscribe(handler?: MessageHandlerFn) {
        Realtime.get().subscribeChannel(this.channel, handler);
    }

    async addHandler(handler: MessageHandlerFn) {
        Realtime.get().registerHandler(this.channel, handler);
    }

    send(payload: object = {}) {
        return Realtime.get().send(this.channel, payload);
    }
}
