import { getPlayers } from "./TUMLiveVjs";
import { VideoJsPlayer } from "video.js";

export class TranscriptController {
    static initiatedInstances: Map<string, Promise<TranscriptController>> = new Map<
        string,
        Promise<TranscriptController>
    >();

    private list: VTTCue[];
    private elem: HTMLElement;
    private lastSyncTime: number;
    private player: VideoJsPlayer;
    private selectedTrackLabel: string;

    reset(): void {
        this.player = getPlayers()[0];
    }

    constructor() {
        this.lastSyncTime = 0;
        this.selectedTrackLabel = "English";
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
        const transcriptDesktop = document.getElementById('transcript-desktop');
        if (!this.elem || !transcriptDesktop || transcriptDesktop.offsetParent === null) {
            return;
        }

        const now = Date.now();
        if (now - this.lastSyncTime < 1000) {
            return;
        }
        this.lastSyncTime = now;

        if (this.player.paused()) {
            return;
        }

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
        for (let i = 0; i < textTracks.length; i++) {
            const track = textTracks[i];
            if ((track.kind === "captions" || track.kind === "subtitles") && track.label === this.selectedTrackLabel) {
                for (let j = 0; j < track.cues.length; j++) {
                    const cue = track.cues[j] as VTTCue;
                    transcript.push(cue);
                }
                if (transcript.length > 0) {
                    return transcript;
                }
            }
        }

        // If no selected track is found, use other available subtitles
        for (let i = 0; i < textTracks.length; i++) {
            const track = textTracks[i];
            if (track.kind === "captions" || track.kind === "subtitles") {
                for (let j = 0; j < track.cues.length; j++) {
                    const cue = track.cues[j] as VTTCue;
                    transcript.push(cue);
                }
            }
        }

        return transcript;
    }

    updateTranscript(transcript: VTTCue[]) {
        this.list = transcript;
        const event = new CustomEvent('update', { detail: transcript });
        this.elem.dispatchEvent(event);
    }

    highlightActiveCue(currentTime: number) {
        const activeCue = this.list.find(cue => cue.startTime <= currentTime && cue.endTime >= currentTime);
        const cueElements = this.elem.querySelectorAll('[data-cue-start]');
        cueElements.forEach((cueElement: HTMLElement) => {
            cueElement.classList.remove('bg-blue-100', 'dark:bg-blue-700');
        });

        if (activeCue) {
            const cueElement = this.elem.querySelector(`[data-cue-start="${activeCue.startTime}"]`);
            if (cueElement) {
                cueElement.classList.add('bg-blue-100', 'dark:bg-blue-700');
                cueElement.scrollIntoView({ behavior: 'smooth', block: 'center' });
            }
        }
    }

    onUpdate(data: any) {
        // Process the data and update the transcript list
        this.updateTranscript(data);
    }

    length(): number {
        return this.list !== undefined ? this.list.length : 0;
    }

    async downloadTranscript() {
        const player = getPlayers()[0];
        const textTracks = player.textTracks();
        let transcript = "";

        // Iterate over the text tracks to find the selected track
        for (let i = 0; i < textTracks.length; i++) {
            const track = textTracks[i];
            if ((track.kind === "captions" || track.kind === "subtitles") && track.label === this.selectedTrackLabel) {
                // Iterate over the cues to extract the transcript text
                for (let j = 0; j < track.cues.length; j++) {
                    const cue = track.cues[j];
                    transcript += `${cue.text}\n\n`;
                }
            }
        }

        // If no selected track is found, use other available subtitles
        if (transcript === "") {
            for (let i = 0; i < textTracks.length; i++) {
                const track = textTracks[i];
                if (track.kind === "captions" || track.kind === "subtitles") {
                    // Iterate over the cues to extract the transcript text
                    for (let j = 0; j < track.cues.length; j++) {
                        const cue = track.cues[j];
                        transcript += `${cue.text}\n\n`;
                    }
                }
            }
        }

        // Create a Blob from the transcript text
        const blob = new Blob([transcript], { type: "text/plain" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = "transcript";
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    }
}