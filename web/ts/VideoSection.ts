import { Delete, postData } from "./global";

export class VideoSection {
    private readonly streamID: number;

    existingSections: section[];
    newSections: section[];
    current: section;
    unsavedChanges: boolean;

    constructor(streamID) {
        this.newSections = [];
        this.existingSections = [];
        this.streamID = streamID;
        this.unsavedChanges = false;
        this.resetCurrent();
        this.load();
    }

    load() {
        return fetch(`/api/stream/${this.streamID}/sections`)
            .then((res) => res.json())
            .then((sections) => {
                if (sections === undefined || sections === null) {
                    this.existingSections = [];
                } else {
                    this.existingSections = sections;
                }
            });
    }
    pushNewSection() {
        this.newSections.push({ ...this.current });
        this.resetCurrent();
        this.unsavedChanges = true;
    }
    removeNewSection(section: section) {
        this.newSections = this.newSections.filter((s) => s !== section);
        this.unsavedChanges = true;
    }
    publishNewSections() {
        postData(`/api/stream/${this.streamID}/sections`, this.newSections).then(async () => {
            await this.load(); // load sections again to avaid js-sorting
            this.newSections = [];
        });
        this.unsavedChanges = false;
    }
    removeExistingSection(id: number) {
        Delete(`/api/stream/${this.streamID}/sections/${id}`).then(async () => {
            await this.load();
        });
    }
    timeStringAsString(section): string {
        let s = "";

        if (section.startHours > 0) {
            s += section.startHours;
            s += ":";
        }
        if (section.startMinutes < 10) {
            s += `0${section.startMinutes}`;
        } else {
            s += section.startMinutes;
        }
        s += ":";
        if (section.startSeconds < 10) {
            s += `0${section.startSeconds}`;
        } else {
            s += section.startSeconds;
        }
        return s;
    }
    private resetCurrent() {
        this.current = {
            description: "",
            startHours: 0,
            startMinutes: 0,
            startSeconds: 0,
            streamID: this.streamID,
        };
    }
}

// TypeScript Mapping of model.VideoSection
type section = {
    description: string;

    startHours: number;
    startMinutes: number;
    startSeconds: number;

    streamID: number;
};
