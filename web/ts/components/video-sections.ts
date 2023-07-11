import { AlpineComponent } from "./alpine-component";
import { Section } from "../api/video-sections";
import { DataStore } from "../data-store/data-store";
import { SlidingWindow } from "../utilities/sliding-window";

export function videoSectionContext(streamId: number): AlpineComponent {
    return {
        streamId: streamId,
        sections: new SlidingWindow([], 6),

        init() {
            console.log("hello from video sections");
            DataStore.videoSections.subscribe(this.streamId, this.updateSection.bind(this));
        },

        updateSection(sections: Section[]) {
            this.sections.set(sections);
            this.sections.reset();
        },
    } as AlpineComponent;
}
