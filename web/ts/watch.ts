import { getPlayers } from "./TUMLiveVjs";
import { copyToClipboard, Time } from "./global";
import { seekbarOverlay } from "./seekbar-overlay";

export enum SidebarState {
    Hidden = "hidden",
    Chat = "chat",
    Bookmarks = "bookmarks",
    Streams = "streams",
}

/*
 MISC
 */

export function contextMenuHandler(e, contextMenu, videoElem) {
    if (contextMenu.shown) return contextMenu;
    e.preventDefault();
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
        const player = getPlayers()[0];
        const vhs = player.tech({ IWillNotUseThisInPlugins: true })["vhs"];
        const notAvailable = vhs == null;

        const data = {
            bufferSeconds: notAvailable ? 0 : player.bufferedEnd() - player.currentTime(),
            videoHeight: notAvailable ? 0 : vhs.playlists.media().attributes.RESOLUTION.height,
            videoWidth: notAvailable ? 0 : vhs.playlists.media().attributes.RESOLUTION.width,
            bandwidth: notAvailable ? 0 : vhs.bandwidth,
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

export function toggleShortcutsModal() {
    const el = document.getElementById("shortcuts-help-modal");
    if (el !== undefined) {
        if (el.classList.contains("hidden")) {
            el.classList.remove("hidden");
        } else {
            el.classList.add("hidden");
        }
    }
}

export class ShareURL {
    url: string;
    includeTimestamp: boolean;
    timestamp: string;

    copied: boolean; // success indicator

    private baseUrl: string;
    private playerHasTime: Promise<boolean>;
    private timestampArgument: string;

    constructor() {
        this.baseUrl = [location.protocol, "//", location.host, location.pathname].join(""); // get rid of query
        this.url = this.baseUrl;
        this.includeTimestamp = false;
        this.copied = false;

        const player = getPlayers()[0];
        player.ready(() => {
            player.on("loadedmetadata", () => {
                this.playerHasTime = Promise.resolve(true);
            });
        });
    }

    async setURL(shouldFetchPlayerTime?: boolean) {
        if (this.includeTimestamp) {
            if (shouldFetchPlayerTime || !this.timestamp) {
                const player = getPlayers()[0];
                await this.playerHasTime;
                this.timestamp = Time.FromSeconds(player.currentTime()).toStringWithLeadingZeros();
                await this.updateURLStateFromTimestamp();
            } else {
                await this.updateURLStateFromTimestamp();
            }
            this.url = this.baseUrl + this.timestampArgument;
        } else {
            this.url = this.baseUrl;
        }
    }

    copyURL() {
        copyToClipboard(this.url);
        this.copied = true;
        setTimeout(() => (this.copied = false), 1000);
    }

    private async updateURLStateFromTimestamp() {
        const trim = this.timestamp.substring(0, 9);
        const split = trim.split(":");
        if (split.length != 3) {
            this.url = this.baseUrl;
        } else {
            const h = +split[0];
            const m = +split[1];
            const s = +split[2];
            if (isNaN(h) || isNaN(m) || isNaN(s) || h > 60 || m > 60 || s > 60 || h < 0 || m < 0 || s < 0) {
                this.url = this.baseUrl;
            } else {
                const inSeconds = s + 60 * m + 60 * 60 * h;
                this.timestampArgument = `?t=${inSeconds}`;
            }
        }
    }
}

export { repeatHeatMap } from "./repeat-heatmap";
export { seekbarHighlights, MarkerType } from "./seekbar-highlights";
export { seekbarOverlay, SeekbarHoverPosition } from "./seekbar-overlay";
export { StreamPlaylist } from "./stream-playlist";
