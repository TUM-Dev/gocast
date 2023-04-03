import { Section } from "./global";
import { getPlayers } from "./TUMLiveVjs";
import { VideoJsPlayer } from "video.js";
import { DataStore } from "./data-store/data-store";
import {Bookmark} from "./data-store/bookmarks";

export enum MarkerType {
    sectionSep,
    bookmark,
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
                return this.setup();
            }
            this.player.one("loadedmetadata", () => this.setup());
        });
    }

    async setup() {
        await DataStore.videoSections.subscribe(this.streamId, (sections) => {
            this.updateSections(sections);
            this.triggerUpdateEvent();
        });
        await DataStore.bookmarks.subscribe(this.streamId, (bookmarks) => {
            this.updateBookmarks(bookmarks);
            this.triggerUpdateEvent();
        });
    }

    async updateSections(sections: Section[]) {
        const duration = this.player.duration();
        this.sections = [];
        this.marker = this.marker.filter((m) => m.type != MarkerType.sectionSep);

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

    async updateBookmarks(bookmarks: Bookmark[]) {
        const duration = this.player.duration();
        this.marker = this.marker.filter((m) => m.type != MarkerType.bookmark);

        for (let i = 0; i < bookmarks.length; i++) {
            const bookmark = bookmarks[i];
            this.marker.push({
                type: MarkerType.bookmark,
                position: this.getBookmarkTimestamp(bookmark) / duration,
            });
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

    getBookmarkTimestamp(bookmark: Bookmark): number {
        return (bookmark.hours * 60 + bookmark.minutes) * 60 + bookmark.seconds;
    }
}

export const seekbarHighlights = new SeekbarHighlights();
