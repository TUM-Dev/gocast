import { Delete, patchData, postData, putData, sendFormData, showMessage, uploadFile } from "./global";
import { StatusCodes } from "http-status-codes";
import { DataStore } from "./data-store/data-store";
import {
    AdminLectureList,
    Lecture,
    LectureVideoType,
    LectureVideoTypeCam,
    LectureVideoTypeComb,
    LectureVideoTypePres,
    LectureVideoTypes,
} from "./api/admin-lecture-list";
import { ChangeSet } from "./change-set";
import { AlpineComponent } from "./components/alpine-component";

export enum UIEditMode {
    none,
    single,
    series,
}

export enum FileType {
    invalid,
    vod,
    attachment,
    image_jpg,
    thumb_comb,
    thumb_cam,
    thumb_pres,
}

export class LectureList {
    courseId: number;
    lectures: Lecture[] = [];
    markedIds: number[] = [];

    constructor(courseId: number) {
        this.courseId = courseId;
        this.markedIds = this.parseUrlHash();
        this.setup();
    }

    async setup() {
        await DataStore.adminLectureList.subscribe(this.courseId, (lectures) => {
            this.lectures = lectures;
            this.triggerUpdateEvent();
        });
    }

    scrollSelectedLecturesIntoView() {
        if (this.markedIds.length > 0) {
            document.querySelector(`#lecture-${this.markedIds[0]}`).scrollIntoView({ behavior: "smooth" });
        }
    }

    parseUrlHash(): number[] {
        if (!window.location.hash.startsWith("#lectures:")) {
            return [];
        }
        return window.location.hash
            .substring("#lectures:".length)
            .split(",")
            .map((s) => parseInt(s));
    }

    hasIndividualChatEnabledSettings(): boolean {
        const lectures = this.lectures;
        if (lectures.length < 2) return false;
        const first = lectures[0];
        return lectures.slice(1).some((l) => l.isChatEnabled !== first.isChatEnabled);
    }

    triggerUpdateEvent() {
        const event = new CustomEvent("newlectures", {
            detail: {
                lectures: this.lectures,
                markedIds: this.markedIds,
            },
        });
        window.dispatchEvent(event);
    }
}

class LectureFile {
    readonly id: number;
    readonly fileType: number;
    readonly friendlyName: string;

    constructor({ id, fileType, friendlyName }) {
        this.id = id;
        this.fileType = fileType;
        this.friendlyName = friendlyName;
    }
}

class DownloadableVod {
    downloadURL: string;
    friendlyName: string;
}

interface VideoFileUI {
    info: LectureVideoType;
    title: string;
    inputId: string;
}

export function lectureEditor(lecture: Lecture): AlpineComponent {
    return {
        videoFiles: [
            { info: LectureVideoTypeComb, title: "Combined Video", inputId: `input_${lecture.lectureId}_comb` },
            { info: LectureVideoTypePres, title: "Presentation Video", inputId: `input_${lecture.lectureId}_pres` },
            { info: LectureVideoTypeCam, title: "Camera Video", inputId: `input_${lecture.lectureId}_cam` },
        ] as VideoFileUI[],

        // UI Data
        lastErrors: [] as string[],
        uiEditMode: UIEditMode.none,
        isDirty: false,
        isSaving: false,

        // Lecture Data
        changeSet: null as ChangeSet<Lecture> | null,
        lectureData: null as Lecture | null,

        /**
         * AlpineJS init function which is called automatically in addition to 'x-init'
         */
        init() {
            // This tracks changes that are not saved yet
            this.changeSet = new ChangeSet<Lecture>(lecture, this.lectureComparator, (data, dirtyState) => {
                this.lectureData = data;
                this.isDirty = dirtyState.isDirty;
            });

            // This updates the state live in background
            DataStore.adminLectureList.subscribe(lecture.courseId, (lectureList) => {
                const update = lectureList.find((l) => l.lectureId === lecture.lectureId);
                if (update) {
                    this.changeSet.updateState(update);
                }
            });
        },

        /**
         * A custom comparator to setup special comparison strategies for specific keys
         * @param key key in Lecture
         * @param a
         * @param b
         * @return return 0 to use naive default comparator
         */
        lectureComparator(key: string, a: Lecture, b: Lecture): boolean | null {
            // here we can set some custom comparisons
            return null;
        },

        toggleVisibility() {
            DataStore.adminLectureList.setPrivate(
                this.lectureData.courseId,
                this.lectureData.lectureId,
                !this.lectureData.private,
            );
        },

        async keepProgressesUpdated() {
            if (!this.isConverting) {
                return;
            }
            setTimeout(async () => {
                for (let i = 0; i < this.transcodingProgresses.length; i++) {
                    const res = await AdminLectureList.getTranscodingProgress(
                        this.lectureData.courseId,
                        this.lectureData.lectureId,
                        this.transcodingProgresses[i].version,
                    );
                    if (res === 100) {
                        this.transcodingProgresses.splice(i, 1);
                    } else {
                        this.transcodingProgresses[i].progress = res;
                    }
                }
                this.isConverting = this.transcodingProgresses.length > 0;
                this.keepProgressesUpdated();
            }, 5000);
        },

        getVideoFile(key: string): File {
            return this.lectureData[key];
        },

        /**
         * Opens the series lecture editor UI
         */
        startSeriesEdit() {
            if (this.uiEditMode !== UIEditMode.none) return;
            this.changeSet.reset();
            this.uiEditMode = UIEditMode.series;
        },

        /**
         * Opens the single lecture editor UI
         */
        startSingleEdit() {
            if (this.uiEditMode !== UIEditMode.none) return;
            this.changeSet.reset();
            this.uiEditMode = UIEditMode.single;
        },

        /**
         * Discards current changes
         */
        discardEdit() {
            this.changeSet.reset();
            this.uiEditMode = UIEditMode.none;
        },

        /**
         * Save changes send them to backend and commit change set.
         */
        async saveEdit() {
            const { courseId, lectureId, name, description, lectureHallId, isChatEnabled } = this.lectureData;
            const changedKeys = this.changeSet.changedKeys();

            try {
                // Saving new meta data
                await DataStore.adminLectureList.updateMeta(courseId, lectureId, {
                    payload: {
                        name: changedKeys.includes("name") ? name : undefined,
                        description: changedKeys.includes("description") ? description : undefined,
                        lectureHallId: changedKeys.includes("lectureHallId") ? lectureHallId : undefined,
                        isChatEnabled: changedKeys.includes("isChatEnabled") ? isChatEnabled : undefined,
                    },
                    options: {
                        saveSeries: this.uiEditMode === UIEditMode.series,
                    },
                });

                // Uploading new videos
                for (const videoFile of this.videoFiles) {
                    if (!changedKeys.includes(videoFile.info.key)) {
                        continue;
                    }

                    const file = this.lectureData[videoFile.info.key];
                    await DataStore.adminLectureList.uploadVideo(courseId, lectureId, videoFile.info.type, file, {
                        onProgress: (progress) => {
                            window.dispatchEvent(
                                new CustomEvent(`voduploadprogressedit`, {
                                    detail: { type: videoFile.info.type, progress, lectureId: this.lectureId },
                                }),
                            );
                        },
                    });
                }
            } catch (e) {
                console.error(e);
                this.lastErrors = [e.message];
                return;
            }

            this.changeSet.commit({ discardKeys: this.videoFiles.map((v) => v.info.key) });
            this.uiEditMode = UIEditMode.none;
        },
    } as AlpineComponent;
}

export function decodeHtml(html) {
    const txt = document.createElement("textarea");
    txt.innerHTML = html;
    return txt.value;
}

export async function deleteLectures(cid: number, lids: number[]) {
    if (confirm("Confirm deleting " + lids.length + " video" + (lids.length == 1 ? "" : "s") + "?")) {
        const res = await postData("/api/course/" + cid + "/deleteLectures", {
            streamIDs: lids.map((n) => n.toString()),
        });

        if (res.status !== StatusCodes.OK) {
            alert("An unknown error occurred during the deletion process!");
            return;
        }

        //LectureList.lectures = LectureList.lectures.filter((l) => !lids.includes(l.lectureId));
        //LectureList.triggerUpdate();
    }
}

export function saveIsChatEnabled(streamId: number, isChatEnabled: boolean) {
    return patchData("/api/stream/" + streamId + "/chat/enabled", { streamId, isChatEnabled });
}

export async function saveIsChatEnabledForAllLectures(isChatEnabled: boolean) {
    const promises = [];
    /*for (const lecture of LectureList.lectures) {
        promises.push(saveIsChatEnabled(lecture.lectureId, isChatEnabled));
    }*/
    const errors = (await Promise.all(promises)).filter((res) => res.status !== StatusCodes.OK);
    return errors.length <= 0;
}

export function saveLectureHall(streamIds: number[], lectureHall: string) {
    return postData("/api/setLectureHall", { streamIds, lectureHall: parseInt(lectureHall) });
}

// Used by schedule.ts
export function saveLectureDescription(e: Event, cID: number, lID: number): Promise<boolean> {
    e.preventDefault();
    const input = (document.getElementById("lectureDescriptionInput" + lID) as HTMLInputElement).value;
    return putData("/api/course/" + cID + "/updateDescription/" + lID, { name: input }).then((res) => {
        if (res.status !== StatusCodes.OK) {
            return false;
        }
        return true;
    });
}

// Used by schedule.ts
export function saveLectureName(e: Event, cID: number, lID: number): Promise<boolean> {
    e.preventDefault();
    const input = (document.getElementById("lectureNameInput" + lID) as HTMLInputElement).value;
    return postData("/api/course/" + cID + "/renameLecture/" + lID, { name: input }).then((res) => {
        if (res.status !== StatusCodes.OK) {
            return false;
        }
        return true;
    });
}

export function showStats(id: number): void {
    if (document.getElementById("statsBox" + id).classList.contains("hidden")) {
        document.getElementById("statsBox" + id).classList.remove("hidden");
    } else {
        document.getElementById("statsBox" + id).classList.add("hidden");
    }
}

export function toggleExtraInfos(btn: HTMLElement, id: number) {
    btn.classList.add("transform", "transition", "duration-500", "ease-in-out");
    if (btn.classList.contains("rotate-180")) {
        btn.classList.remove("rotate-180");
        document.getElementById("extraInfos" + id).classList.add("hidden");
    } else {
        btn.classList.add("rotate-180");
        document.getElementById("extraInfos" + id).classList.remove("hidden");
    }
}

export function showHideUnits(id: number) {
    const container = document.getElementById("unitsContainer" + id);
    if (container.classList.contains("hidden")) {
        container.classList.remove("hidden");
    } else {
        container.classList.add("hidden");
    }
}

interface MediaUpload {
    type: string;
    file: File;
    progress: number;
}

export enum LectureCreateType {
    vodRecord,
    livestream,
    vodUpload,
}

function stopRecorder(recorder: MediaRecorder): Promise<Blob> {
    return new Promise((resolve) => {
        recorder.ondataavailable = (e) => {
            resolve(e.data);
        };
        recorder.stop();
    });
}

function loadVideoBlob(elem: HTMLVideoElement, video: Blob): Promise<void> {
    return new Promise((resolve) => {
        elem.srcObject = null;
        elem.onloadedmetadata = (e) => {
            elem.pause();
            elem.currentTime = 0;
            resolve();
        };
        elem.src = URL.createObjectURL(video);
    });
}

class LectureRecorder {
    private eventRoot: HTMLElement;
    private readonly onUpdateData: (screenRecording: Blob, cameraRecording: Blob) => void;

    private screencastStream: MediaStream;
    private cameraStream: MediaStream;

    private screencastDisplay: HTMLVideoElement;
    private cameraDisplay: HTMLVideoElement;

    private screencastRecorder: MediaRecorder;
    private cameraRecorder: MediaRecorder;

    private screenRecording: Blob;
    private cameraRecording: Blob;

    public screencastAvailable: boolean;
    public cameraAvailable: boolean;
    public isRecording: boolean;
    public retrieveRecording: boolean;
    public recordingsReady: boolean;
    public isPlaying: boolean;

    constructor(eventRoot: HTMLElement, onUpdateData: (screenRecording: Blob, cameraRecording: Blob) => void) {
        this.eventRoot = eventRoot;
        this.onUpdateData = onUpdateData;
        this.screencastAvailable = false;
        this.cameraAvailable = false;
        this.isRecording = false;
        this.retrieveRecording = false;
        this.recordingsReady = false;
        this.isPlaying = false;
    }

    async selectScreencast(display: HTMLVideoElement): Promise<void> {
        if (this.isRecording || this.recordingsReady) return;
        try {
            this.screencastDisplay = display;
            this.screencastStream = await navigator.mediaDevices.getDisplayMedia({
                audio: true,
                video: true,
            });

            await this.initScreencastDisplay();
        } catch (err) {
            alert("Failed to access your screen.");
        }
    }

    async selectCamera(display: HTMLVideoElement): Promise<void> {
        if (this.isRecording || this.recordingsReady) return;
        try {
            this.cameraDisplay = display;
            this.cameraStream = await navigator.mediaDevices.getUserMedia({
                audio: true,
                video: true,
            });

            await this.initCameraDisplay();

            this.cameraAvailable = true;
        } catch (err) {
            alert("Failed to access your webcam & mic.");
        }
    }

    initScreencastDisplay() {
        return new Promise<void>((resolve) => {
            this.screencastDisplay.srcObject = this.screencastStream;
            this.screencastDisplay.onloadedmetadata = (e) => {
                this.screencastDisplay.muted = true;
                this.screencastDisplay.play();
                this.screencastRecorder = new MediaRecorder(this.screencastStream, {
                    mimeType: "video/webm",
                });
                this.screencastAvailable = true;
                resolve();
            };
        });
    }

    initCameraDisplay(): Promise<void> {
        return new Promise<void>((resolve) => {
            this.cameraDisplay.srcObject = this.cameraStream;
            this.cameraDisplay.onloadedmetadata = (e) => {
                this.cameraDisplay.muted = true;
                this.cameraDisplay.play();
                this.cameraRecorder = new MediaRecorder(this.cameraStream, {
                    mimeType: "video/webm",
                });
                resolve();
            };
        });
    }

    async toggleRecording(): Promise<void> {
        if (this.isRecording) {
            await this.stop();
        } else {
            this.start();
        }
    }

    start(): void {
        if (this.isRecording) return;
        if (this.screencastAvailable) {
            this.screencastRecorder.start();
        }
        if (this.cameraRecorder) {
            this.cameraRecorder.start();
        }
        this.isRecording = true;
    }

    async stop(): Promise<void> {
        if (!this.isRecording || this.retrieveRecording) return;
        this.isRecording = false;
        this.retrieveRecording = true;

        await Promise.all(
            [
                async () => {
                    if (!this.screencastRecorder) {
                        return;
                    }
                    this.screenRecording = await stopRecorder(this.screencastRecorder);
                    this.screencastRecorder = null;
                },
                async () => {
                    if (!this.cameraRecorder) {
                        return;
                    }
                    this.cameraRecording = await stopRecorder(this.cameraRecorder);
                    this.cameraRecorder = null;
                },
            ].map((fn) => fn()),
        );
        await this.displayRecordings();
        this.onUpdateData(this.screenRecording, this.cameraRecording);
        this.retrieveRecording = false;
        this.recordingsReady = true;
    }

    async displayRecordings(): Promise<void> {
        if (this.screenRecording) {
            await loadVideoBlob(this.screencastDisplay, this.screenRecording);
        }
        if (this.cameraRecording) {
            await loadVideoBlob(this.cameraDisplay, this.cameraRecording);
        }
    }

    togglePlay() {
        if (this.isPlaying) {
            this.pause();
        } else {
            this.play();
        }
    }

    play() {
        if (this.isPlaying) return;
        if (this.screenRecording) {
            this.screencastDisplay.play();
            this.screencastDisplay.onended = () => this.resetPlay();
        }
        if (this.cameraRecording) {
            this.cameraDisplay.currentTime = this.screencastDisplay.currentTime;
            this.cameraDisplay.play();
            this.cameraDisplay.onended = () => this.resetPlay();
        }
        this.isPlaying = true;
    }

    pause() {
        if (!this.isPlaying) return;
        if (this.screenRecording) {
            this.screencastDisplay.pause();
        }
        if (this.cameraRecording) {
            this.cameraDisplay.pause();
        }
        this.isPlaying = false;
    }

    resetPlay() {
        this.pause();
        if (this.screenRecording) {
            this.screencastDisplay.currentTime = 0;
        }
        if (this.cameraRecording) {
            this.cameraDisplay.currentTime = 0;
        }
    }

    deleteRecordings() {
        this.pause();
        if (confirm("Are your sure you want to delete all recordings?")) {
            this.screenRecording = null;
            this.cameraRecording = null;
            this.recordingsReady = null;
            this.initScreencastDisplay();
            this.initCameraDisplay();
            this.onUpdateData(this.screenRecording, this.cameraRecording);
        }
    }
}

export function createLectureForm(args: { s: [] }) {
    return {
        createType: LectureCreateType.livestream,
        currentTab: 0,
        canGoBack: false,
        canContinue: true,
        onLastSlide: false,
        formData: {
            title: "",
            lectureHallId: 0,
            start: "",
            end: "",
            isChatEnabled: false,
            duration: 0, // Duration in Minutes
            formatedDuration: "", // Duration in Minutes
            premiere: false,
            vodup: false,
            adHoc: false,
            recurring: false,
            recurringInterval: "weekly",
            eventsCount: 10,
            recurringDates: [],
            combFile: [],
            presFile: [],
            camFile: [],
        },
        streams: args.s,
        loading: false,
        error: false,
        courseID: -1,
        invalidReason: "",
        init() {
            this.onUpdate();
        },
        initRecorder(eventRoot: HTMLElement): LectureRecorder {
            return new LectureRecorder(eventRoot, (screen, cam) => this.updateRecordings(screen, cam));
        },
        next() {
            if (this.onLastSlide) {
                this.submitData();
                return;
            }
            this.currentTab++;
            this.onUpdate();
        },
        prev() {
            this.currentTab--;
            this.onUpdate();
        },
        updateCreateType(newType: LectureCreateType) {
            this.createType = newType;
            if (newType === LectureCreateType.livestream) {
                this.formData.vodup = false;
            } else {
                this.formData.vodup = true;
            }
        },
        updateLiveAdHoc(adHoc: boolean) {
            this.formData.adHoc = adHoc;
            this.next();
        },
        updateType(vodup: boolean) {
            this.formData.vodup = vodup;
            if (vodup) {
                this.formData.recurring = false;
            }
            this.next();
        },
        onStartChange() {
            setTimeout(() => {
                this.regenerateRecurringDates();
                this.recalculateDuration();
                this.onUpdate();
            }, 100);
        },
        onEndChange() {
            setTimeout(() => {
                this.recalculateDuration();
                this.onUpdate();
            }, 100);
        },

        updateFiles(type: string, file: File[]) {
            if (type === "COMB") {
                this.formData.combFile = file;
            } else if (type === "CAM") {
                this.formData.camFile = file;
            } else if (type === "PRES") {
                this.formData.presFile = file;
            }
            this.onUpdate();
        },

        updateRecordings(screenRecording: Blob, cameraRecording: Blob) {
            this.formData.combFile = [];
            this.formData.presFile = [];
            this.formData.camFile = [];

            if (screenRecording) {
                this.formData.presFile = [new File([screenRecording], "pres.webm")];
            }
            if (cameraRecording) {
                this.formData.camFile = [new File([cameraRecording], "cam.webm")];
            }

            this.onUpdate();
        },

        // This function sets flags depending on the current tab and current data
        onUpdate() {
            if (this.currentTab === 0) {
                this.canContinue = true;
                this.canGoBack = false;
                this.onLastSlide = false;
                return;
            }

            if (this.currentTab === 1) {
                this.onLastSlide = false;
                if (this.formData.vodup) {
                    // If user has chosen video on demand, there are 3 tabs (file upload tab)
                    // => we are not on the last tab
                    this.canGoBack = true;
                    this.canContinue = this.formData.start.length > 0;
                } else {
                    this.onLastSlide = true;
                    this.canGoBack = true;
                    this.canContinue = this.formData.start.length > 0 && this.formData.end.length > 0;
                }
                return;
            }

            if (this.currentTab === 2) {
                this.canContinue =
                    (this.getMediaFiles().length > 0 && this.formData.vodup) ||
                    (this.formData.adHoc && this.formData.end != "");
                this.canGoBack = true;
                this.onLastSlide = true;
                return;
            }
        },
        validateForm() {
            this.invalidReason = "";
            const hasDupes =
                this.streams.filter((s) => {
                    return s == this.formData.start;
                }).length !== 0;
            if (hasDupes) {
                this.invalidReason = "A lecture on this date and time already exists.";
            }
        },
        regenerateRecurringDates() {
            const result = [];
            if (this.formData.start != "") {
                for (let i = 0; i < this.formData.eventsCount - 1; i++) {
                    const date = i == 0 ? new Date(this.formData.start) : new Date(result[i - 1].date);
                    switch (this.formData.recurringInterval) {
                        case "daily":
                            date.setDate(date.getDate() + 1);
                            break;
                        case "weekly":
                            date.setDate(date.getDate() + 7);
                            break;
                        case "monthly":
                            date.setMonth(date.getMonth() + 1);
                            break;
                    }
                    result.push({
                        date,
                        enabled: true,
                    });
                }
            }
            this.formData.recurringDates = result;
        },
        recalculateDuration() {
            if (this.formData.adHoc) {
                this.formData.start = new Date().toISOString();
            }
            if (this.formData.start != "" && this.formData.end != "") {
                const [hours, minutes] = this.formData.end.split(":");
                const startDate = new Date(this.formData.start);
                const endDate = new Date(this.formData.start);
                endDate.setHours(hours);
                endDate.setMinutes(minutes);
                if (endDate.getTime() <= startDate.getTime()) {
                    endDate.setDate(endDate.getDate() + 1);
                }
                this.formData.duration = (endDate.getTime() - startDate.getTime()) / 1000 / 60;
            } else {
                this.formData.duration = 0;
            }
            this.generateFormatedDuration();
        },
        generateFormatedDuration() {
            const hours = Math.floor(this.formData.duration / 60);
            const minutes = this.formData.duration - hours * 60;
            let res = "";
            if (hours > 0) {
                res += `${hours}h `;
            }
            if (minutes > 0) {
                res += `${minutes}min`;
            }
            this.formData.formatedDuration = res;
        },
        async submitData() {
            const currentTab = this.currentTab;
            this.currentTab = -1;
            this.loading = true;

            if (this.formData.vodup) {
                try {
                    const streamId = await this.uploadVod();
                    const url = new URL(window.location.href);
                    url.hash = `lectures:${streamId}`;
                    window.location.assign(url);
                    window.location.reload();
                } catch (e) {
                    this.currentTab = currentTab;
                    this.loading = false;
                    this.error = true;
                }
            } else {
                const payload = {
                    title: this.formData.title,
                    lectureHallId: this.formData.lectureHallId.toString(),
                    premiere: this.formData.premiere,
                    vodup: this.formData.vodup,
                    start: this.formData.start,
                    adHoc: this.formData.adHoc,
                    duration: this.formData.duration,
                    isChatEnabled: this.formData.isChatEnabled,
                    dateSeries: [],
                    // todo: file: undefined,
                };
                if (this.formData.recurring) {
                    for (const date of this.formData.recurringDates.filter(({ enabled }) => enabled)) {
                        payload.dateSeries.push(date.date.toISOString());
                    }
                }
                if (this.formData.premiere || this.formData.vodup) {
                    // todo: payload.file = this.formData.file[0];
                    payload.duration = 0; // premieres have no explicit end set -> use "0" here
                }
                postData("/api/course/" + this.courseID + "/createLecture", payload)
                    .then(async (res) => {
                        const { ids } = await res.json();
                        /*const url = new URL(window.location.href);
                        url.hash = `lectures:${ids.join(",")}`;
                        window.location.assign(url);
                        window.location.reload();*/
                    })
                    .catch((e) => {
                        console.log(e);
                        this.currentTab = currentTab;
                        this.loading = false;
                        this.error = true;
                    });
            }
        },

        getMediaFiles(): MediaUpload[] {
            const mediaUploads: MediaUpload[] = [];
            if (this.formData.combFile[0] != null)
                mediaUploads.push({ file: this.formData.combFile[0], type: "COMB", progress: 0 });
            if (this.formData.presFile[0] != null)
                mediaUploads.push({ file: this.formData.presFile[0], type: "PRES", progress: 0 });
            if (this.formData.camFile[0] != null)
                mediaUploads.push({ file: this.formData.camFile[0], type: "CAM", progress: 0 });
            return mediaUploads;
        },

        dispatchMediaProgress(mediaFiles: MediaUpload[]) {
            const detail = {
                COMB: mediaFiles.find((e) => e.type == "COMB")?.progress ?? null,
                PRES: mediaFiles.find((e) => e.type == "PRES")?.progress ?? null,
                CAM: mediaFiles.find((e) => e.type == "CAM")?.progress ?? null,
            };
            window.dispatchEvent(new CustomEvent("voduploadprogress", { detail }));
        },

        async uploadVod(): Promise<number> {
            const mediaFiles = this.getMediaFiles();
            this.dispatchMediaProgress(mediaFiles);

            // Create New VOD
            const { streamID } = await (
                await postData(
                    `/api/course/${this.courseID}/createVOD?start=${this.formData.start}&title=${this.formData.title}`,
                    {},
                )
            ).json();

            // Upload media
            try {
                for (const mediaUpload of mediaFiles) {
                    await uploadFile(
                        `/api/course/${this.courseID}/uploadVODMedia?streamID=${streamID}&videoType=${mediaUpload.type}`,
                        mediaUpload.file,
                        {
                            onProgress: (progress) => {
                                mediaUpload.progress = progress;
                                this.dispatchMediaProgress(mediaFiles);
                            },
                        },
                    );
                }
            } catch (e) {
                await postData(`/api/course/${this.courseID}/deleteLectures`, {
                    streamIDs: [streamID],
                });
                throw e;
            }

            return streamID;
        },
    };
}

export function sendCourseSettingsForm(courseId: number) {
    const form = document.getElementById("course-settings-form") as HTMLFormElement;
    const formData = new FormData(form);
    sendFormData(`/admin/course/${courseId}`, formData);
}

export async function submitFormAndEnableAllIndividualChats(courseId: number, isChatEnabled: boolean) {
    const res = await saveIsChatEnabledForAllLectures(isChatEnabled);
    sendCourseSettingsForm(courseId);
    return res;
}

export function deleteCourse(courseID: string) {
    if (confirm("Do you really want to delete this course? This includes all associated lectures.")) {
        const url = `/api/course/${courseID}/`;
        fetch(url, { method: "DELETE" }).then((res) => {
            if (!res.ok) {
                alert("Couldn't delete course.");
            } else {
                window.location.replace("/admin");
            }
        });
    }
}

export function copyCourse(courseID: string, year: string, yearW: string, semester: string) {
    const url = `/api/course/${courseID}/copy`;
    fetch(url, { method: "POST", body: JSON.stringify({ year, yearW, semester }) }).then((res) => {
        if (!res.ok) {
            alert("Couldn't copy course.");
        } else {
            res.json().then((r) => window.location.replace(`/admin/course/${r.newCourse}?copied`));
        }
    });
}
