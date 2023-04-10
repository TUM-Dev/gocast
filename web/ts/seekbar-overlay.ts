import { getPlayers } from "./TUMLiveVjs";
import { cloneEvents } from "./global";

export class SeekbarHoverPosition {
    static empty: SeekbarHoverPosition = new SeekbarHoverPosition(0,0);

    private readonly seekBarWidth: number;
    public position: number;
    public offset: number;

    constructor(seekBarWidth: number, offset: number) {
        this.seekBarWidth = seekBarWidth;
        this.offset = offset;
        this.position = offset / seekBarWidth;
    }

    public onTarget(target: number, maxDeltaPixels: number) : boolean {
        if (this.offset === -1) return false;
        const deltaPercentage = maxDeltaPixels / this.seekBarWidth;
        return this.position >= (target - deltaPercentage) && this.position <= (target + deltaPercentage);
    }

    public inRange(from: number, to: number) {
        if (this.offset === -1) return false;
        return this.position >= from && this.position < to;
    }
}

export const seekbarOverlay = {
    streamID: null,
    outerWrap: null,
    seekBarWrap: null,

    init(wrap: HTMLElement) {
        this.outerWrap = wrap;
        const player = [...getPlayers()].pop();
        player.ready(() => {
            this.seekBarWrap = player.el().querySelector(".vjs-progress-control");
            const slider = this.seekBarWrap.querySelector(".vjs-slider");
            slider.style.marginLeft = 0;
            slider.style.marginRight = 0;
            cloneEvents(slider, this.seekBarWrap, ["mousemove", "mouseleave"]);
            cloneEvents(this.outerWrap, this.seekBarWrap, ["mousemove", "mouseleave"]);
            this.injectElementIntoVjs();
            this.updateSize();
            this.listenForHoverEvents();
            new ResizeObserver(this.updateSize.bind(this)).observe(this.seekBarWrap);
        });
    },

    listenForHoverEvents() {
        this.seekBarWrap.addEventListener("mousemove", (e: MouseEvent) => {
            if (e.target !== this.seekBarWrap) return;
            this.triggerHoverEvent(new SeekbarHoverPosition(this.seekBarWrap.getBoundingClientRect().width, e.offsetX));
        });
        this.seekBarWrap.addEventListener("mouseleave", (e: MouseEvent) => {
            if (e.target !== this.seekBarWrap) return;
            this.triggerHoverEvent(new SeekbarHoverPosition(this.seekBarWrap.getBoundingClientRect().width, -1));
        });
    },

    triggerHoverEvent(pos: SeekbarHoverPosition) {
        const event = new CustomEvent("seekbarhover", {
            detail: pos,
        });
        window.dispatchEvent(event);
    },

    injectElementIntoVjs() {
        const heatmap = document.querySelector(".seekbar-overlay");
        this.seekBarWrap.append(heatmap);
    },

    updateSize() {
        const event = new CustomEvent("updateseekbarsize", {
            detail: this.getSeekbarInfo(),
        });
        window.dispatchEvent(event);
    },

    getSeekbarInfo() {
        const seekBar = this.seekBarWrap.querySelector(".vjs-progress-holder");
        if (!seekBar) {
            return { x: "0px", width: "0px" };
        }

        const marginLeft = window.getComputedStyle(seekBar).marginLeft;
        const width = seekBar.getBoundingClientRect().width;
        return {
            x: marginLeft,
            width: width + "px",
        };
    },
};
