import videojs, {VideoJsPlayer} from "video.js";

export class Segment {
    constructor(public start: number, public end: number, public del: boolean, focussed: boolean) {}
}

export function editor() {
    return {
        player: undefined,
        initPlayer(video: HTMLVideoElement) {
            this.player = videojs(video, {
                controls: false,
                fluid: true,
            });
            const that = this;
            this.player.on("timeupdate", function () {
                that.timestamp = this.currentTime()/this.duration();
            });
            this.getWaveform();
        },
        setPos(pos:number) {
            this.player.currentTime(this.player.duration()*pos);
            this.timestamp = pos;
        },
        getWaveform() {
            let waveform = document.getElementById("waveform") as HTMLImageElement;
            waveform.src = "/api/editor/waveform?video="+(this.player as VideoJsPlayer).src();
        },
        addCut() {
            // find segment index that is active:
            let index = 0;
            this.segments.forEach((segment, i) => {
                if (segment.start <= this.timestamp && segment.end >= this.timestamp) {
                    index = i;
                }
            });
            // split segment at timestamp
            let segment = this.segments[index];
            segment.focussed = false;
            let newSegment = new Segment(this.timestamp, segment.end, false, true);
            segment.end = this.timestamp;
            this.segments.splice(index+1, 0, newSegment);
        },
        deleteCurrentSegment() {
            // find segment index that is active:
            let index = 0;
            this.segments.forEach((segment, i) => {
                if (segment.start <= this.timestamp && segment.end >= this.timestamp) {
                    index = i;
                }
            });
            this.segments[index].del = true;
        },
        zoom: 100,
        timestamp:0,
        prevTimestamp:0,
        segments: [new Segment(0,1,false, true)],
    }
}
