import { getPlayer } from "./player";
import { copyToClipboard } from "./global";

export class ShareURL {
    private baseUrl: string;

    url: string;
    includeTimestamp: boolean;
    timestamp: string;
    openTime: number;

    copied: boolean; // success indicator

    constructor() {
        this.baseUrl = [location.protocol, "//", location.host, location.pathname].join(""); // get rid of query
        this.url = this.baseUrl;
        this.includeTimestamp = false;
        this.copied = false;

        const player = getPlayer();
        player.ready(() => {
            player.on("loadedmetadata", () => {
                this.openTime = player.currentTime();
            });
        });
    }

    copyURL() {
        copyToClipboard(this.url);
        this.copied = true;
        setTimeout(() => (this.copied = false), 3000);
    }

    setURL() {
        if (this.includeTimestamp) {
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
                    this.url = `${this.baseUrl}?t=${inSeconds}`;
                }
            }
        } else {
            this.url = this.baseUrl;
        }
    }

    setTimestamp() {
        const d = new Date(this.openTime * 1000);
        const h = ShareURL.padZero(d.getUTCHours());
        const m = ShareURL.padZero(d.getUTCMinutes());
        const s = ShareURL.padZero(d.getSeconds());
        this.timestamp = `${h}:${m}:${s}`;
    }

    private static padZero(i) {
        if (i < 10) {
            i = "0" + i;
        }
        return i;
    }
}
