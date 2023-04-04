import { Section, Time } from "./global";
import { DataStore } from "./data-store/data-store";
import { UpdateVideoSectionRequest } from "./data-store/video-sections";
import {Bookmark} from "./data-store/bookmarks";

export abstract class VideoSectionList {
    private streamId: number;

    protected list: Section[];

    currentHighlightIndex: number;

    protected constructor(streamId: number) {
        this.streamId = streamId;
        this.list = [];
        this.currentHighlightIndex = -1;

        DataStore.videoSections.subscribe(this.streamId, (data) => this.onUpdate(data));
    }

    async fetch() {

    }

    private onUpdate(data: Section[]) {
        this.list = data;
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

    private listener;

    constructor(streamId: number) {
        this.streamId = streamId;

        this.newSections = [];
        this.existingSections = [];
        this.unsavedChanges = false;
        this.resetCurrent();

        DataStore.videoSections.subscribe(this.streamId, (data) => this.onUpdate(data));
    }

    async fetch() {

    }

    onUpdate(data: Section[]) {
        this.existingSections = data;
    }

    pushNewSection() {
        this.current.friendlyTimestamp = new Time(
            this.current.startHours,
            this.current.startMinutes,
            this.current.startSeconds,
        ).toString();
        this.newSections.push({ ...this.current });
        this.resetCurrent();
        this.unsavedChanges = true;
    }

    removeNewSection(section: Section) {
        this.newSections = this.newSections.filter((s) => s !== section);
        this.unsavedChanges = true;
    }

    async publishNewSections() {
        await DataStore.videoSections.add(this.streamId, this.newSections);
        this.newSections = [];
        this.unsavedChanges = false;
    }

    async removeExistingSection(id: number) {
        await DataStore.videoSections.delete(this.streamId, id);
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
        await DataStore.videoSections.update(this.streamId, this.section.ID, this.request);
        this.show = false;
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
