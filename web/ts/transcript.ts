import { getPlayers } from "./TUMLiveVjs";
import videojs from "video.js";

type Player = ReturnType<typeof videojs>;

export class TranscriptController {
    static initiatedInstances: Map<string, Promise<TranscriptController>> = new Map();

    private list: VTTCue[];
    private elem: HTMLElement;
    private lastSyncTime: number;
    private player: Player;
    private selectedTrackLabel: string;
    private availableLanguages: string[];

    constructor() {
        this.lastSyncTime = 0;
        this.selectedTrackLabel = "";
        this.availableLanguages = [];
    }

    reset(): void {
        this.player = getPlayers()[0];
    }

    getAvailableLanguages(): string[] {
        if (!this.player) {
            return [];
        }
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const textTracks = this.player.textTracks() as any;
        const languages: string[] = [];
        for (let i = 0; i < textTracks.length; i++) {
            const track = textTracks[i];
            if ((track.kind === "captions" || track.kind === "subtitles") && track.label) {
                if (!languages.includes(track.label)) {
                    languages.push(track.label);
                }
            }
        }
        return languages;
    }

    setLanguage(label: string): void {
        this.selectedTrackLabel = label;
        // Trigger a sync to update the transcript with the new language
        this.syncTranscript();
    }

    getSelectedLanguage(): string {
        return this.selectedTrackLabel;
    }

    async init(key: string, element: HTMLElement) {
        if (TranscriptController.initiatedInstances[key]) {
            (await TranscriptController.initiatedInstances[key]).unsub();
        }
        TranscriptController.initiatedInstances[key] = new Promise<TranscriptController>(() => {
            this.elem = element;
        });

        this.player = getPlayers()[0];

        // Initialize with the first available language
        this.availableLanguages = this.getAvailableLanguages();
        if (this.availableLanguages.length > 0 && !this.selectedTrackLabel) {
            this.selectedTrackLabel = this.availableLanguages[0];
        }

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

    async fetchTranscript(player: Player): Promise<VTTCue[]> {
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

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    getTranscriptFromTracks(textTracks: any, label?: string): VTTCue[] {
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

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    getTranscriptText(textTracks: any, label?: string): string {
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
