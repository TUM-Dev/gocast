import { SeekLogger } from "./player";
import { startWebsocket } from "./chat-interactions";

export const watchpage = (streamID: number) => ({
    streamID: streamID,
    showChat: false,
    showShare: false,
    seekLogger: null,
    showBookmarks: false,
    showShortcuts: false,

    init() {
        this.showChat = this.$persist(false).as("chatOpen");
        this.seekLogger = new SeekLogger(this.streamID);
        this.seekLogger.attach();
        startWebsocket(this.streamID);
    },

    onShift(e) {
        if (document.activeElement.tagName !== "INPUT" && document.activeElement.tagName !== "TEXTAREA") {
            switch (e.key) {
                case "?": {
                    this.toggleShortcuts();
                }
            }
        }
    },

    toggleShortcuts() {
        this.showShortcuts = !this.showShortcuts;
    },
});
