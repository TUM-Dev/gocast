import videojs, { VideoJsPlayer } from "video.js";

export class Segment {
    constructor(public start: number, public end: number, public del: boolean, focussed: boolean) {}
}

export function editor(courseID: number, streamID: number) {
    return {
        courseID: courseID,
        streamID: streamID,
        player: undefined,
        debounced: 0,
        showHelp: false,
        initPlayer(video: HTMLVideoElement) {
            this.player = videojs(video, {
                controls: false,
                fluid: true,
            });
            this.player.on("timeupdate", ( ) => {
                this.timestamp = this.player.currentTime() / this.player.duration();
            });
            this.getWaveform();
            document.addEventListener("keydown", (e) => {
                // debounce so events won't be triggered twice
                if (this.debounced > new Date().getTime()) {
                    return;
                }
                this.debounced = new Date().getTime() + 100;
                switch (e.key) {
                    case "ArrowLeft":
                        this.player.currentTime(this.player.currentTime() - 1/30);
                        break;
                    case "ArrowRight":
                        this.player.currentTime(this.player.currentTime() + 1/30);
                        break;
                    case " ":
                        this.player.paused() ? this.player.play() : this.player.pause();
                        break;
                    case "d":
                    case "Delete":
                        this.toggleDeleteCurrentSegment();
                        break;
                    case "r":
                        this.restoreCurrentSegment();
                        break;
                    case "c":
                        this.addCut();
                        break;
                    case "+":
                        this.zoom = Math.min(300, this.zoom + 10);
                        break;
                    case "-":
                        this.zoom = Math.max(100, this.zoom - 10);
                        break;
                }
            });
        },
        mergeRight() {
            const i = this.getCurrentSegmentIndex();
            if (i == this.segments.length-1){
                return;
            }
            this.segments[i].end = this.segments[i+1].end;
            this.segments.splice(i+1, 1);
        },
        mergeLeft() {
            const i = this.getCurrentSegmentIndex();
            if (i == 0){
                return;
            }
            this.segments[i].start = this.segments[i-1].start;
            this.segments.splice(i-1, 1);
        },
        setPos(pos: number) {
            if (Number.isNaN(pos)) {
                return;
            }
            this.player.currentTime(this.player.duration() * pos);
            this.timestamp = pos;
        },
        getWaveform() {
            const waveform = document.getElementById("waveform") as HTMLImageElement;
            waveform.src = "/api/editor/waveform?video=" + (this.player as VideoJsPlayer).src();
        },
        addCut() {
            const i = this.getCurrentSegmentIndex();
            // split segment at timestamp
            const segment = this.segments[i];
            segment.focussed = false;
            const newSegment = new Segment(this.timestamp, segment.end, false, true);
            segment.end = this.timestamp;
            this.segments.splice(i + 1, 0, newSegment);
        },
        toggleDeleteCurrentSegment() {
            // this can be simplified, but then it doesn't work for some reason
            if (this.segments[this.getCurrentSegmentIndex()].del){
                this.segments[this.getCurrentSegmentIndex()].del = false;
            } else {
                this.segments[this.getCurrentSegmentIndex()].del = true;
            }
        },
        getCurrentSegmentIndex() {
            let index = 0;
            this.segments.forEach((segment, i) => {
                if (segment.start <= this.timestamp && segment.end >= this.timestamp) {
                    index = i;
                }
            });
            return index;
        },
        restoreCurrentSegment() {
            this.segments[this.getCurrentSegmentIndex()].del = false;
        },
        submit() {
            fetch(`/api/editor/${this.courseID}/${this.streamID}`, {
                method: "POST",
                body: JSON.stringify(this.segments),
            }).then((response) => response.json());
        },
        zoom: 100,
        timestamp: 0,
        prevTimestamp: 0,
        segments: [new Segment(0, 1, false, true)],
        clickPos(e: MouseEvent) {
            this.setPos(e.offsetX / (e.target as HTMLImageElement).width);
        },
        click(timestamp: number) {
            this.setPos(timestamp);
        },
        movePos(e: MouseEvent) {
            this.prevTimestamp = e.offsetX / (e.target as HTMLImageElement).width;
        },
    };
}
