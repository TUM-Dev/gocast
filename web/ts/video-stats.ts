import { getPlayer } from "./player";

export function contextMenuHandler(e, contextMenu) {
    if (contextMenu.shown) return contextMenu;
    e.preventDefault();
    const videoElem = document.querySelector("#my-video");
    return {
        shown: true,
        locX: e.clientX - videoElem.getBoundingClientRect().left,
        locY: e.clientY - videoElem.getBoundingClientRect().top,
    };
}

export const videoStatListener = {
    videoStatIntervalId: null,
    listen() {
        if (this.videoStatIntervalId != null) {
            return;
        }
        this.videoStatIntervalId = setInterval(this.update, 1000);
        this.update();
    },
    update() {
        const player = getPlayer();
        const vhs = player.tech({ IWillNotUseThisInPlugins: true }).vhs;
        const notAvailable = vhs == null;

        const data = {
            bufferSeconds: notAvailable ? 0 : player.bufferedEnd() - player.currentTime(),
            videoHeight: notAvailable ? 0 : vhs.playlists.media().attributes.RESOLUTION.height,
            videoWidth: notAvailable ? 0 : vhs.playlists.media().attributes.RESOLUTION.width,
            bandwidth: notAvailable ? 0 : vhs.bandwidth, //player.tech().vhs.bandwidth(),
            mediaRequests: notAvailable ? 0 : vhs.stats.mediaRequests,
            mediaRequestsFailed: notAvailable ? 0 : vhs.stats.mediaRequestsErrored,
        };
        const event = new CustomEvent("newvideostats", { detail: data });
        window.dispatchEvent(event);
    },
    clear() {
        if (this.videoStatIntervalId != null) {
            clearInterval(this.videoStatIntervalId);
            this.videoStatIntervalId = null;
        }
    },
};
