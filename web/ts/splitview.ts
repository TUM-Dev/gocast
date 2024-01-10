import { getPlayers } from "./TUMLiveVjs";
import Split from "split.js";
import { cloneEvents } from "./global";
import videojs, {VideoJsPlayer} from "video.js";
import PlayerOptions = videojs.PlayerOptions;

const mouseMovingTimeout = 2200;

export class SplitView {
    private camPercentage: number;
    /* eslint-disable  @typescript-eslint/no-explicit-any */
    private players: any[];
    private split: Split.Instance;
    private gutterWidth = 10;
    private isFullscreen = false;
    private splitParent: HTMLElement;
    private videoWrapper: HTMLElement;
    private videoWrapperResizeObs: ResizeObserver;

    showSplitMenu: boolean;
    mouseMoving: boolean;

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
        this.mouseMoving = false;
        this.players = getPlayers();
        this.splitParent = document.querySelector("#video-pres-wrapper").parentElement;
        this.videoWrapper = document.querySelector(".splitview-wrap");
        this.videoWrapperResizeObs = new ResizeObserver(() => this.onVideoWrapperResize());
        this.videoWrapperResizeObs.observe(this.videoWrapper);
        this.detectMouseNotMoving();

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

    onVideoWrapperResize() {
        this.updateControlBarSize(this.getSizes());
    }

    updateMouseMoving(isMoving: boolean) {
        if (isMoving != this.mouseMoving) {
            this.mouseMoving = isMoving;
            this.videoWrapper.dispatchEvent(new CustomEvent("updateMouseMoving", { detail: this.mouseMoving }));
        }
    }

    detectMouseNotMoving() {
        let mouseNotMovingTimeout;
        this.videoWrapper.addEventListener("mousemove", (e) => {
            this.updateMouseMoving(true);

            clearTimeout(mouseNotMovingTimeout);
            mouseNotMovingTimeout = setTimeout(() => {
                this.updateMouseMoving(false);
            }, mouseMovingTimeout);
        });
        this.videoWrapper.addEventListener("mouseleave", (e) => {
            clearTimeout(mouseNotMovingTimeout);
            this.updateMouseMoving(false);
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

        this.updateControlBarSize(this.getSizes());
    }

    private updateControlBarSize(sizes: number[]) {
        const wrapperSize = this.videoWrapper.getBoundingClientRect().width;

        let marginLeft;
        if (this.isFullscreen) {
            marginLeft = "0";
        } else if (sizes[0] === 100) {
            marginLeft = `${this.gutterWidth / 2 - wrapperSize}px`; //`calc(${this.gutterWidth / 2}px - 100vw)`;
        } else if (sizes[0] === 0) {
            marginLeft = `-${this.gutterWidth / 2}px`;
        } else {
            const leftContainerWidth = (sizes[0] * wrapperSize) / 100;
            marginLeft = `-${leftContainerWidth}px`;
        }

        const mainControlBarElem = this.players[1].controlBar.el();
        mainControlBarElem.style.marginLeft = marginLeft;
        mainControlBarElem.style.width = `${wrapperSize}px`;

        const textTrackDisplay = this.players[1].el_.querySelector(".vjs-text-track-display");
        if (textTrackDisplay) {
            textTrackDisplay.style.left = marginLeft;
        }
    }

    private overwriteFullscreenToggle() {
        const fullscreenToggle = this.players[1].controlBar.fullscreenToggle;
        fullscreenToggle.off("click");

        fullscreenToggle.on("click", async () => {
            await this.toggleFullscreen();
        });

        (this.players[0] as VideoJsPlayer).options_.userActions.doubleClick = async () => await this.toggleFullscreen();
        (this.players[1] as VideoJsPlayer).options_.userActions.doubleClick = async () => await this.toggleFullscreen();

        this.splitParent.addEventListener("fullscreenchange", () => this.update(25))
    }

    private async toggleFullscreen() {
        if (document.fullscreenElement === null) {
            await this.splitParent.requestFullscreen();
        } else {
            await document.exitFullscreen();
        }
    }

    private setTrackBarModes(k: number, mode: string) {
        const tracks = this.players[k].textTracks();
        for (let i = 0; i < tracks.length; i++) {
            tracks[i].mode = mode;
        }
    }
}
