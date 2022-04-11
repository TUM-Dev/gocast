import {Delete, postData} from "./global";

export class VideoSection {
    private readonly streamID: number;

    existingSections: section[];
    newSections: section[];
    current: section;

    constructor(streamID) {
        this.newSections = [];
        this.existingSections = [];
        this.streamID = streamID;
        this.resetCurrent();
    }
    async load() {
        await fetch(`/api/stream/${this.streamID}/sections`)
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
    }
    removeNewSection(section: section) {
        this.newSections = this.newSections.filter((s) => s !== section);
    }
    publishNewSections() {
        postData(`/api/stream/${this.streamID}/sections`, this.newSections).then(async () => {
            await this.load(); // load sections again to avaid js-sorting
            this.newSections = [];
        });
    }
    removeExistingSection(id: number) {
        Delete(`/api/stream/${this.streamID}/sections/${id}`).then(async () => {
            await this.load();
        });
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
