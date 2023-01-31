import { getPlayers } from "./TUMLiveVjs";
import Split from "split.js";
import { cloneEvents } from "./global";

export class SplitView {
    private camPercentage: number;
    private players: any[];
    private split: Split.Instance;
    private gutterWidth = 10;
    private isFullscreen = false;

    private splitParent: HTMLElement;

    showSplitMenu: boolean;

    static Options = {
        FullPresentation: 0,
        FocusPresentation: 25,
        SplitEvenly: 50,
        FocusCamera: 75,
        FullCamera: 100,
    };

    constructor() {
        this.camPercentage = SplitView.Options.FocusPresentation;
        this.showSplitMenu = false;
        this.players = getPlayers();
        this.splitParent = document.querySelector("#video-pres-wrapper").parentElement;

        this.players[0].ready(() => {
            this.setTrackBarModes(0, "disabled");
        });

        this.players[1].ready(() => {
            this.setupControlBars();
            this.overwriteFullscreenToggle();
        });

        cloneEvents(this.players[0].el(), this.players[1].el(), ["mousemove", "mouseenter", "mouseleave"]);

        // Setup splitview
        // eslint-disable-next-line @typescript-eslint/no-this-alias
        const that = this;
        this.split = Split(["#video-pres-wrapper", "#video-cam-wrapper"], {
            minSize: [0, 0],
            sizes: this.getSizes(),
            onDrag(sizes: number[]) {
                that.updateControlBarSize(sizes);
            },
        });
    }

    update(percentage: number) {
        this.camPercentage = percentage;
        const newSizes = this.getSizes();
        this.split.setSizes(newSizes);
        this.updateControlBarSize(newSizes);
    }

    hideMenu() {
        this.showSplitMenu = false;
    }

    toggleMenu() {
        this.showSplitMenu = !this.showSplitMenu;
    }

    private getSizes(): number[] {
        return [100 - this.camPercentage, this.camPercentage];
    }

    private setupControlBars() {
        this.players[0].controlBar.hide();
        this.players[0].muted(true);

        this.players[1].el().addEventListener("fullscreenchange", () => {
            this.isFullscreen = document.fullscreenElement !== null;
            this.updateControlBarSize(this.getSizes());
        });

        const mainControlBarElem = this.players[1].controlBar.el();
        mainControlBarElem.style.position = "absolute";
        mainControlBarElem.style.zIndex = "1";
        mainControlBarElem.style.width = "100vw";

        this.updateControlBarSize(this.getSizes());
    }

    private updateControlBarSize(sizes: number[]) {
        let newSize;
        if (this.isFullscreen) {
            newSize = "0";
        } else if (sizes[0] === 100) {
            newSize = `calc(${this.gutterWidth / 2}px - 100vw)`;
        } else if (sizes[0] === 0) {
            newSize = `-${this.gutterWidth / 2}px`;
        } else {
            newSize = `-${sizes[0]}vw`;
        }

        this.players[1].controlBar.el_.style.marginLeft = newSize;

        const textTrackDisplay = this.players[1].el_.querySelector(".vjs-text-track-display");
        if (textTrackDisplay) {
            textTrackDisplay.style.left = newSize;
        }
    }

    private overwriteFullscreenToggle() {
        const fullscreenToggle = this.players[1].controlBar.fullscreenToggle;
        fullscreenToggle.off("click");

        fullscreenToggle.on("click", async () => {
            if (document.fullscreenElement === null) {
                await this.splitParent.requestFullscreen();
            } else {
                await document.exitFullscreen();
            }
        });
    }

    private setTrackBarModes(k: number, mode: string) {
        const tracks = this.players[k].textTracks();
        for (let i = 0; i < tracks.length; i++) {
            tracks[i].mode = mode;
        }
    }
}
