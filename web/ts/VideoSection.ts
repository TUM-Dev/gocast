import { postData } from "./global";

export class VideoSection {
    readonly streamID: number;

    newSections: Section[];
    current: Section;

    constructor(streamID) {
        this.newSections = [];
        this.streamID = streamID;
        this.current = new Section(streamID);
    }
    pushSection() {
        this.newSections.push(this.current);
        this.current = new Section(this.streamID);
    }
    remove(section: Section) {
        this.newSections = this.newSections.filter((s) => s !== section);
    }
    add() {
        const sections = [];
        this.newSections.forEach((s) => sections.push(s.actual));
        postData(`/api/stream/${this.streamID}/sections`, this.newSections).then(() => {
            console.log("added");
        });
    }
}

class Section {
    actual: section;
    hours: number;
    minutes: number;
    seconds: number;

    constructor(streamID: number) {
        this.actual = {
            description: "",
            startInSeconds: 0,
            streamID: streamID,
        };
        this.hours = 0;
        this.minutes = 0;
        this.seconds = 0;
    }
    get(): section {
        this.actual.startInSeconds = this.hours * 60 * 60 + this.minutes * 60 + this.seconds;
        return this.actual;
    }
}

// TypeScript Mapping of model.VideoSection
type section = {
    description: string;
    startInSeconds: number;
    streamID: number;
};
