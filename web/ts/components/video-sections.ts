import { AlpineComponent } from "./alpine-component";
import { Section } from "../api/video-sections";
import { DataStore } from "../data-store/data-store";
import { SlidingWindow } from "../utilities/sliding-window";
import { registerTimeWatcher } from "../video/watchers";
import { getPlayers } from "../TUMLiveVjs";
import { Time } from "../utilities/time";
import { ToggleableElement } from "../utilities/ToggleableElement";

export function videoSectionContext(streamId: number): AlpineComponent {
    return {
        streamId: streamId,
        autoScroll: new ToggleableElement(),
        sections: new SlidingWindow([], 6),

        init() {
            DataStore.videoSections.subscribe(this.streamId, this.updateSection.bind(this));
            registerTimeWatcher(getPlayers()[0], this.setCurrent.bind(this));
        },

        nextSection() {
            this.autoScroll.toggle(false);
            this.sections.next();
        },

        prevSection() {
            this.autoScroll.toggle(false);
            this.sections.prev();
        },

        setCurrent(t: number) {
            this.sections.forEach((s, _) => (s.isCurrent = false));
            const section = this.sections.find((s, i, arr) => {
                const next = arr[i + 1];
                const sectionSeconds = new Time(s.startHours, s.startMinutes, s.startSeconds).toSeconds();
                return next === undefined || next === null // if last element and no next exists
                    ? sectionSeconds <= t
                    : sectionSeconds <= t &&
                          t <= new Time(next.startHours, next.startMinutes, next.startSeconds).toSeconds() - 1;
            });

            if (section) {
                section.isCurrent = true;
                if (!this.sections.isInWindow(section) && this.autoScroll.value) this.sections.show(section);
            }
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
