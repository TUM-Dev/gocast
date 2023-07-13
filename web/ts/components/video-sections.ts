import { AlpineComponent } from "./alpine-component";
import { Section } from "../api/video-sections";
import { DataStore } from "../data-store/data-store";
import { SlidingWindow } from "../utilities/sliding-window";
import { registerTimeWatcher } from "../video/watchers";
import { getPlayers } from "../TUMLiveVjs";
import { Time } from "../utilities/time";

export function videoSectionContext(streamId: number): AlpineComponent {
    return {
        streamId: streamId,
        sections: new SlidingWindow([], 6),

        init() {
            DataStore.videoSections.subscribe(this.streamId, this.updateSection.bind(this));
            registerTimeWatcher(getPlayers()[0], this.setCurrent.bind(this));
        },

        setCurrent(t: number) {
            this.sections.forEach((s, _) => (s.isCurrent = false));
            const section: Section = this.sections.find(
                (s, _) => new Time(s.startHours, s.startMinutes, s.startSeconds).toSeconds() >= t,
            );

            if (section) section.isCurrent = true;

            // if (!this.sections.isInWindow(section)) this.sections.slideToWindowFor(section);
        },

        isCurrent(i: number) {
            return i == 0;
        },

        updateSection(sections: Section[]) {
            this.sections.set(sections);
            this.sections.reset();
        },
    } as AlpineComponent;
}
