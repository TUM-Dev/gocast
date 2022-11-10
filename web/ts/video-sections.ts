import { Delete, getData, postData, putData, Section } from "./global";

export abstract class VideoSectionList {
    private streamId: number;

    protected list: Section[];

    currentHighlightIndex: number;

    protected constructor(streamId: number) {
        this.streamId = streamId;
        this.list = [];
        this.currentHighlightIndex = -1;
    }

    async fetch() {
        VideoSections.get(this.streamId).then((list) => {
            this.list = list;
        });
    }

    abstract getList(): Section[];

    abstract isCurrent(i: number): boolean;
}

/**
 * Mobile VideoSection Functionality
 * @category watch-page
 */
export class VideoSectionsMobile extends VideoSectionList {
    minimize: boolean;

    constructor(streamId: number) {
        super(streamId);
        this.minimize = true;
    }

    getList(): Section[] {
        return this.minimize && this.list.length > 0 ? [this.list.at(this.currentHighlightIndex)] : this.list;
    }

    isCurrent(i: number): boolean {
        return this.minimize ? true : this.currentHighlightIndex !== -1 && i === this.currentHighlightIndex;
    }
}

/**
 * Desktop VideoSection Functionality
 * @category watch-page
 */
export class VideoSectionsDesktop extends VideoSectionList {
    readonly sectionsPerGroup: number;

    private followSections: boolean;
    private currentIndex: number;

    constructor(streamId: number) {
        super(streamId);
        this.currentIndex = 0;
        this.followSections = false;
        this.sectionsPerGroup = 4;
    }

    getList(): Section[] {
        const currentHighlightPage = Math.floor(this.currentHighlightIndex / this.sectionsPerGroup);
        const startIndex = this.followSections && this.validHighlightIndex() ? currentHighlightPage : this.currentIndex;
        return this.list.slice(
            startIndex * this.sectionsPerGroup,
            startIndex * this.sectionsPerGroup + this.sectionsPerGroup,
        );
    }

    isCurrent(i: number): boolean {
        const idx =
            this.currentHighlightIndex -
            Math.floor(this.currentHighlightIndex / this.sectionsPerGroup) * this.sectionsPerGroup;
        return this.validHighlightIndex() && this.onCurrentPage() && i === idx;
    }

    showNext(): boolean {
        return this.currentIndex < this.list.length / this.sectionsPerGroup - 1;
    }

    showPrev(): boolean {
        return this.currentIndex > 0;
    }

    next() {
        this.currentIndex = (this.currentIndex + 1) % this.list.length;
    }

    prev() {
        this.currentIndex = (this.currentIndex - 1) % this.list.length;
    }

    private validHighlightIndex(): boolean {
        return this.currentHighlightIndex !== -1;
    }

    private onCurrentPage(): boolean {
        const currentHighlightPage = Math.floor(this.currentHighlightIndex / this.sectionsPerGroup);
        return (
            (this.followSections ? currentHighlightPage : this.currentIndex) ===
            Math.floor(this.currentHighlightIndex / this.sectionsPerGroup)
        );
    }
}

/**
 * Admin Page VideoSection Management
 * @category admin-page
 */
export class VideoSectionsAdmin {
    private readonly streamId: number;

    existingSections: Section[];
    newSections: Section[];
    current: Section;
    unsavedChanges: boolean;

    constructor(streamId: number) {
        this.streamId = streamId;

        this.newSections = [];
        this.existingSections = [];
        this.unsavedChanges = false;
        this.resetCurrent();
    }

    async fetch() {
        VideoSections.get(this.streamId).then((list) => {
            this.existingSections = list;
        });
    }

    pushNewSection() {
        this.current.friendlyTimestamp = timeStringAsString(this.current);
        this.newSections.push({ ...this.current });
        this.resetCurrent();
        this.unsavedChanges = true;
    }

    removeNewSection(section: Section) {
        this.newSections = this.newSections.filter((s) => s !== section);
        this.unsavedChanges = true;
    }

    publishNewSections() {
        VideoSections.add(this.streamId, this.newSections).then(async () => {
            await this.fetch(); // load sections again to avoid js-sorting
            this.newSections = [];
        });
        this.unsavedChanges = false;
    }

    removeExistingSection(id: number) {
        VideoSections.delete(this.streamId, id).then(() => {
            this.existingSections = this.existingSections.filter((s) => s.ID !== id);
        });
    }

    private resetCurrent() {
        this.current = {
            description: "",
            startHours: 0,
            startMinutes: 0,
            startSeconds: 0,
            streamID: this.streamId,
        };
    }
}

function timeStringAsString(section: Section): string {
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

/**
 * Admin Page VideoSection Updater
 * @category admin-page
 */
export class VideoSectionUpdater {
    private readonly streamId: number;
    private section: Section;

    request: UpdateVideoSectionRequest;
    show: boolean;

    constructor(streamId: number, section: Section) {
        this.streamId = streamId;
        this.section = section;
        this.reset();
    }

    async update() {
        return VideoSections.update(this.streamId, this.section.ID, this.request).then(() => {
            // 1.) Update old
            this.section.startHours = this.request.StartHours;
            this.section.startMinutes = this.request.StartMinutes;
            this.section.startSeconds = this.request.StartSeconds;
            this.section.description = this.request.Description;
            this.section.friendlyTimestamp = timeStringAsString(this.section);
            this.show = false;
        });
    }

    reset() {
        this.request = new UpdateVideoSectionRequest();
        this.request.Description = this.section.description;
        this.request.StartHours = this.section.startHours;
        this.request.StartMinutes = this.section.startMinutes;
        this.request.StartSeconds = this.section.startSeconds;
        this.show = false;
    }
}

class UpdateVideoSectionRequest {
    Description: string;
    StartHours: number;
    StartMinutes: number;
    StartSeconds: number;
}

/**
 * Wrapper for REST-API calls @ /api/stream/:id/sections
 * @category watch-page
 * @category admin-page
 */
const VideoSections = {
    get: async function (streamId: number): Promise<Section[]> {
        return getData(`/api/stream/${streamId}/sections`)
            .then((resp) => {
                if (!resp.ok) {
                    throw Error(resp.statusText);
                }
                return resp.json();
            })
            .catch((err) => {
                console.error(err);
                return [];
            })
            .then((l: Section[]) => l);
    },

    add: async function (streamId: number, request: object) {
        return postData(`/api/stream/${streamId}/sections`, request);
    },

    update: function (streamId: number, id: number, request: object) {
        return putData(`/api/stream/${streamId}/sections/${id}`, request);
    },

    delete: async function (streamId: number, id: number): Promise<Response> {
        return Delete(`/api/stream/${streamId}/sections/${id}`);
    },
};
