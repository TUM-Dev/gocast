import videojs, {VideoJsPlayer} from "video.js";

export class Segment {
    constructor(public start: number, public end: number, public del: boolean, focussed: boolean) {}
}

export function editor(courseID:number, streamID:number) {
    return {
        courseID: courseID,
        streamID: streamID,
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
        submit() {
            fetch(`/api/editor/${this.courseID}/${this.streamID}`, {method: "POST", body: JSON.stringify(this.segments)})
                .then(response => response.json());
        },
        zoom: 100,
        timestamp:0,
        prevTimestamp:0,
        segments: [new Segment(0,1,false, true)],
        clickPos(e: MouseEvent) {
            console.log(e.offsetX/(e.target as HTMLImageElement).width);
            this.setPos(e.offsetX/(e.target as HTMLImageElement).width);
        },
        movePos(e: MouseEvent) {
            const waveform = e.target as HTMLImageElement;
            const percentage = (e.clientX - waveform.getBoundingClientRect().left) / waveform.clientWidth;
            const zoomedPercentage = percentage * this.zoom / 100;
            this.prevTimestamp = zoomedPercentage;
        },
    }
}
