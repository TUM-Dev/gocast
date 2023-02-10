import { getPlayers } from "./TUMLiveVjs";
import { Get } from "./global";

const repeatMapScale = 90;

export const repeatHeatMap = {
    seekBarWrap: null,

    init(streamID: number) {
        this.streamID = streamID;
        setTimeout(() => this.updateHeatMap(), 0);

        const player = [...getPlayers()].pop();
        player.ready(() => {
            this.seekBarWrap = player.el().querySelector(".vjs-progress-control");
            this.injectElementIntoVjs();

            new ResizeObserver(this.updateSize.bind(this)).observe(this.seekBarWrap);
            this.updateSize();
        });
    },

    injectElementIntoVjs() {
        const heatmap = document.querySelector(".heatmap-wrap");
        this.seekBarWrap.append(heatmap);
    },

    updateSize() {
        const event = new CustomEvent("updateheatmapsize", {
            detail: this.getSeekbarInfo(),
        });
        window.dispatchEvent(event);
    },

    valuesToArray(listOfChunkValues) {
        const max = listOfChunkValues.reduce((res, item) => (res > item.value ? res : item.value), 0);
        return [...Array(100)].map((val, index) => {
            const item = listOfChunkValues.find((e) => e.index === index);
            const size = item ? (repeatMapScale / max) * item.value : 0;
            return repeatMapScale - size;
        });
    },

    updateHeatMap() {
        const values = JSON.parse(Get(`/api/seekReport/${this.streamID}`)).values;
        if (values.length == 0) {
            return;
        }

        const event = new CustomEvent("updateheatmappath", {
            detail: { heatMapPath: this.genHeatMapPath(this.valuesToArray(values)) },
        });

        window.dispatchEvent(event);
    },

    genHeatMapPath(values) {
        let res = "M 0.0,100.0";

        for (let i = 0; i < 1000; i += 10) {
            const currX = i + 1;
            const lastVal = values[i / 10 - 1] ?? 0;
            const currVal = values[i / 10];

            const slope = (lastVal + currVal) / 2;
            const arcHandleA = `${currX.toFixed(1)},${lastVal.toFixed(1)}`;
            const arcHandleB = `${(currX + 2).toFixed(1)},${slope.toFixed(1)}`;
            const to = `${(currX + 6).toFixed(1)},${currVal.toFixed(1)}`;

            const p = ` C ${arcHandleA} ${arcHandleB} ${to}`;
            res += p;
        }

        const lastVal = values[values.length - 1];
        const slope = (lastVal + 100) / 2;

        res += ` C 1001.0,${lastVal} 1000.0,${slope} 1000.0,100.0`;

        return res;
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