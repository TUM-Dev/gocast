import { VideoSections } from "./video-sections";
import { Section } from "./global";
import { getPlayers } from "./TUMLiveVjs";
import { VideoJsPlayer } from "video.js";

export enum MarkerType {
    sectionSep,
}

export type SeekbarMarker = {
    type: MarkerType;
    icon?: string;
    description?: string;
    position: number;
};

export type SeekbarSection = {
    title: string;
    from: number;
    to: number;
};

class SeekbarHighlights {
    streamId: number;
    player: VideoJsPlayer;
    sections: SeekbarSection[];
    marker: SeekbarMarker[];

    init(streamID: number) {
        this.streamId = streamID;
        this.marker = [];
        this.sections = [];
        this.player = [...getPlayers()].pop();
        this.player.ready(() => {
            if (this.player.duration() > 0) {
                return this.update();
            }
            this.player.one("loadedmetadata", () => this.update());
        });
    }

    async update() {
        await this.updateSections();
        this.triggerUpdateEvent();
    }

    async updateSections() {
        const duration = this.player.duration();
        this.sections = [];
        this.marker = this.marker.filter((m) => m.type != MarkerType.sectionSep);

        const sections = await VideoSections.get(this.streamId);
        for (let i = 0; i < sections.length; i++) {
            const section = sections[i];
            const nextSection = i + 1 < sections.length ? sections[i + 1] : null;

            const from = this.getSectionTimestamp(section) / duration;
            const to = nextSection ? this.getSectionTimestamp(nextSection) / duration : 1;

            this.sections.push({
                title: section.description,
                from: from,
                to: to,
            });

            if (to != 1) {
                this.marker.push({
                    type: MarkerType.sectionSep,
                    position: to,
                });
            }
        }
    }

    triggerUpdateEvent() {
        const event = new CustomEvent("seekbarhighlightsupdate", {
            detail: {
                sections: this.sections,
                marker: this.marker,
            },
        });
        window.dispatchEvent(event);
    }

    getSectionTimestamp(section: Section): number {
        return (section.startHours * 60 + section.startMinutes) * 60 + section.startSeconds;
    }
}

export const seekbarHighlights = new SeekbarHighlights();
