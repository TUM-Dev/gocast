import { getPlayers } from "./TUMLiveVjs";
import { Get } from "./global";

export const seekbarOverlay = {
    streamID: null,
    seekBarWrap: null,

    init() {
        const player = [...getPlayers()].pop();
        player.ready(() => {
            this.seekBarWrap = player.el().querySelector(".vjs-progress-control");
            this.injectElementIntoVjs();
            this.updateSize();
            new ResizeObserver(this.updateSize.bind(this)).observe(this.seekBarWrap);
        });
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
