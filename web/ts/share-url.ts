import { getPlayers } from "./TUMLiveVjs";
import { copyToClipboard } from "./global";

export class ShareURL {
    url: string;
    includeTimestamp: boolean;
    timestamp: string;

    copied: boolean; // success indicator

    private readonly baseUrl: string;
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
                await this.setTimestamp(player.currentTime());
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

    private async setTimestamp(time: number) {
        const d = new Date(time * 1000);
        const h = ShareURL.padZero(d.getUTCHours());
        const m = ShareURL.padZero(d.getUTCMinutes());
        const s = ShareURL.padZero(d.getSeconds());
        this.timestamp = `${h}:${m}:${s}`;
    }

    private static padZero(i: string | number) {
        if (i < 10) {
            i = "0" + i;
        }
        return i;
    }
}
