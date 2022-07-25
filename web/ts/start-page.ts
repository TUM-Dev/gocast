import { wsPubSubClient } from "./socket";

export const liveUpdateListener = {
    async init() {
        await wsPubSubClient.subscribeChannel("live-update", this.handle);
    },

    handle(payload: object) {
        window.dispatchEvent(new CustomEvent("liveupdate", { detail: { data: payload } }));
    },
};
