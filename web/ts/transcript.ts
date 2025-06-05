import { getPlayers } from "./TUMLiveVjs";
import { VideoJsPlayer } from "video.js";

export class TranscriptController {
    static initiatedInstances: Map<string, Promise<TranscriptController>> = new Map();

    private list: VTTCue[];
    private elem: HTMLElement;
    private lastSyncTime: number;
    private player: VideoJsPlayer;
    private selectedTrackLabel: string;

    constructor() {
        this.lastSyncTime = 0;
        this.selectedTrackLabel = "English";
    }

    reset(): void {
        this.player = getPlayers()[0];
    }

    async init(key: string, element: HTMLElement) {
        if (TranscriptController.initiatedInstances[key]) {
            (await TranscriptController.initiatedInstances[key]).unsub();
        }
        TranscriptController.initiatedInstances[key] = new Promise<TranscriptController>(() => {
            this.elem = element;
        });

        this.player = getPlayers()[0];
        window.setInterval(() => this.syncTranscript(), 1000);
    }

    async syncTranscript() {
        const transcriptDesktop = document.getElementById("transcript-desktop");
        if (!this.elem || !transcriptDesktop || transcriptDesktop.offsetParent === null) {
            return;
        }

        const now = Date.now();
        // Sync once every second
        if (now - this.lastSyncTime < 1000 || this.player.paused()) {
            return;
        }
        this.lastSyncTime = now;
        console.debug("Syncing transcript...");

        const currentTime = this.player.currentTime();
        const transcript = await this.fetchTranscript(this.player);
        this.updateTranscript(transcript);
        this.highlightActiveCue(currentTime);
    }

    async fetchTranscript(player: VideoJsPlayer): Promise<VTTCue[]> {
        const textTracks = player.textTracks();
        let transcript: VTTCue[] = [];

        // Try to find the selected track first
        transcript = this.getTranscriptFromTracks(textTracks, this.selectedTrackLabel);
        if (transcript.length > 0) {
            return transcript;
        }

        // If no selected track is found, use other available subtitles
        transcript = this.getTranscriptFromTracks(textTracks);
        return transcript;
    }

    getTranscriptFromTracks(textTracks: TextTrackList, label?: string): VTTCue[] {
        const transcript: VTTCue[] = [];
        for (let i = 0; i < textTracks.length; i++) {
            const track = textTracks[i];
            if ((track.kind === "captions" || track.kind === "subtitles") && (!label || track.label === label)) {
                for (let j = 0; j < track.cues.length; j++) {
                    const cue = track.cues[j] as VTTCue;
                    transcript.push(cue);
                }
                if (label && transcript.length > 0) {
                    return transcript;
                }
            }
        }
        return transcript;
    }

    updateTranscript(transcript: VTTCue[]) {
        this.list = transcript;
        const event = new CustomEvent("update", { detail: transcript });
        this.elem.dispatchEvent(event);
    }

    highlightActiveCue(currentTime: number) {
        const activeCue = this.list.find((cue) => cue.startTime <= currentTime && cue.endTime >= currentTime);
        const cueElements = this.elem.querySelectorAll("[data-cue-start]");
        cueElements.forEach((cueElement: HTMLElement) => {
            cueElement.classList.remove("bg-blue-100", "dark:bg-blue-700");
        });

        if (activeCue) {
            const cueElement = this.elem.querySelector(`[data-cue-start="${activeCue.startTime}"]`);
            if (cueElement) {
                cueElement.classList.add("bg-blue-100", "dark:bg-blue-700");
                cueElement.scrollIntoView({ behavior: "smooth", block: "center" });
            }
        }
    }

    onUpdate(data: VTTCue[]) {
        this.updateTranscript(data);
    }

    length(): number {
        return this.list !== undefined ? this.list.length : 0;
    }

    async downloadTranscript() {
        const player = getPlayers()[0];
        const textTracks = player.textTracks();
        let transcript = this.getTranscriptText(textTracks, this.selectedTrackLabel);

        // If no selected track is found, use other available subtitles
        if (transcript === "") {
            transcript = this.getTranscriptText(textTracks);
        }

        this.downloadTextAsFile(transcript, "transcript.txt");
    }

    getTranscriptText(textTracks: TextTrackList, label?: string): string {
        let transcript = "";
        for (let i = 0; i < textTracks.length; i++) {
            const track = textTracks[i];
            if ((track.kind === "captions" || track.kind === "subtitles") && (!label || track.label === label)) {
                for (let j = 0; j < track.cues.length; j++) {
                    const cue = track.cues[j];
                    transcript += `${(cue as VTTCue).text}\n\n`;
                }
                if (label && transcript !== "") {
                    return transcript;
                }
            }
        }
        return transcript;
    }

    downloadTextAsFile(text: string, filename: string) {
        const blob = new Blob([text], { type: "text/plain" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    }
}
