import { Delete, postData, Section } from "./global";

/**
 * Wrapper for REST-API calls @ /api/stream/:id/sections
 * @category watch-page
 * @category admin-page
 */
export class VideoSectionClient {
    readonly streamID: number;

    constructor(streamID: number) {
        this.streamID = streamID;
    }

    async fetch(): Promise<Section[]> {
        return await fetch(`/api/stream/${this.streamID}/sections`).then((res: Response) => {
            if (!res.ok) {
                throw new Error("Could not fetch sections");
            }
            return res.json();
        });
    }

    async post(data): Promise<Response> {
        return postData(`/api/stream/${this.streamID}/sections`, data);
    }

    async delete(id: number): Promise<Response> {
        return Delete(`/api/stream/${this.streamID}/sections/${id}`);
    }
}

export abstract class VideoSections {
    protected list: Section[];

    currentHighlightIndex: number;

    constructor() {
        this.list = [];
        this.currentHighlightIndex = -1;
    }

    async fetch(client: VideoSectionClient) {
        client
            .fetch()
            .then((list) => {
                this.list = list;
            })
            .catch((err) => {
                console.log(err);
                this.list = [];
            });
    }

    abstract getList(): Section[];

    abstract isCurrent(i: number): boolean;
}

/**
 * Mobile VideoSection Functionality
 * @category watch-page
 */
export class VideoSectionsMobile extends VideoSections {
    getList(): Section[] {
        return this.list;
    }

    isCurrent(i: number): boolean {
        return this.currentHighlightIndex !== -1 && i === this.currentHighlightIndex;
    }
}

/**
 * Desktop VideoSection Functionality
 * @category watch-page
 */
export class VideoSectionsDesktop extends VideoSections {
    readonly sectionsPerGroup: number;

    private followSections: boolean;
    private currentIndex: number;

    constructor() {
        super();
        this.currentIndex = 0;
        this.followSections = false;
        this.sectionsPerGroup = 4;
    }

    getList(): Section[] {
        const currentHighlightPage = Math.floor(this.currentHighlightIndex / this.sectionsPerGroup);
        const startIndex = this.followSections ? currentHighlightPage : this.currentIndex;
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
    private readonly streamID: number;

    existingSections: Section[];
    newSections: Section[];
    current: Section;
    unsavedChanges: boolean;

    client: VideoSectionClient;

    constructor(client: VideoSectionClient) {
        this.client = client;

        this.newSections = [];
        this.existingSections = [];
        this.unsavedChanges = false;
        this.resetCurrent();
    }

    async fetch() {
        this.client
            .fetch()
            .then((list) => {
                this.existingSections = list;
            })
            .catch((err) => {
                console.log(err);
                this.existingSections = [];
            });
    }

    pushNewSection() {
        this.current.friendlyTimestamp = VideoSectionsAdmin.timeStringAsString(this.current);
        this.newSections.push({ ...this.current });
        this.resetCurrent();
        this.unsavedChanges = true;
    }

    removeNewSection(section: Section) {
        this.newSections = this.newSections.filter((s) => s !== section);
        this.unsavedChanges = true;
    }

    publishNewSections() {
        this.client.post(this.newSections).then(async () => {
            await this.fetch(); // load sections again to avaid js-sorting
            this.newSections = [];
        });
        this.unsavedChanges = false;
    }

    removeExistingSection(id: number) {
        this.client.delete(id).then(async () => {
            await this.fetch();
        });
    }

    private static timeStringAsString(section: Section): string {
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
