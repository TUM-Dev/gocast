import { SeekLogger } from "./player";
import { startWebsocket } from "./watch";

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
        startWebsocket();
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
