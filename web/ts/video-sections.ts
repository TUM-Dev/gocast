import { Time } from "./global";
import { DataStore } from "./data-store/data-store";
import { Section, UpdateVideoSectionRequest } from "./api/video-sections";

/**
 * Admin Page VideoSection Management
 * @category admin-page
 */
export class VideoSectionsAdminController {
    static initiatedInstances: Map<string, Promise<VideoSectionsAdminController>> = new Map<
        string,
        Promise<VideoSectionsAdminController>
    >();

    private readonly streamId: number;

    existingSections: Section[];
    newSections: Section[];
    current: Section;
    unsavedChanges: boolean;

    private elem: HTMLElement;
    private unsub: () => void;

    constructor(streamId: number) {
        this.streamId = streamId;

        this.newSections = [];
        this.existingSections = [];
        this.unsavedChanges = false;
        this.resetCurrent();
    }

    async init(key: string, element: HTMLElement) {
        if (VideoSectionsAdminController.initiatedInstances[key]) {
            (await VideoSectionsAdminController.initiatedInstances[key]).unsub();
        }

        VideoSectionsAdminController.initiatedInstances[key] = new Promise<VideoSectionsAdminController>((resolve) => {
            this.elem = element;
            const callback = (data) => this.onUpdate(data);
            DataStore.videoSections.subscribe(this.streamId, callback).then(() => {
                this.unsub = () => DataStore.videoSections.unsubscribe(this.streamId, callback);
                resolve(this);
            });
        });
    }

    onUpdate(data: Section[]) {
        this.existingSections = data;
        this.elem.dispatchEvent(new CustomEvent("update", { detail: this.existingSections }));
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
            isCurrent: false,
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
