import { Delete, patchData, postData, putData, sendFormData, showMessage } from "./global";
import { StatusCodes } from "http-status-codes";

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
    static lectures: Lecture[] = [];
    static markedIds: number[] = [];

    static init(initialState: Lecture[]) {
        if (initialState === null) {
            initialState = [];
        }

        this.markedIds = this.parseUrlHash();

        // load initial state into lecture objects
        for (const lecture of initialState) {
            let l = new Lecture();
            l = Object.assign(l, lecture);
            l.start = new Date(lecture.start);
            l.end = new Date(lecture.end);
            LectureList.lectures.push(l);
        }

        LectureList.triggerUpdate();
        setTimeout(() => this.scrollSelectedLecturesIntoView(), 500);
    }

    static scrollSelectedLecturesIntoView() {
        if (this.markedIds.length > 0) {
            document.querySelector(`#lecture-${this.markedIds[0]}`).scrollIntoView({ behavior: "smooth" });
        }
    }

    static parseUrlHash(): number[] {
        if (!window.location.hash.startsWith("#lectures:")) {
            return [];
        }
        return window.location.hash
            .substring("#lectures:".length)
            .split(",")
            .map((s) => parseInt(s));
    }

    static hasIndividualChatEnabledSettings(): boolean {
        const lectures = this.lectures;
        if (lectures.length < 2) return false;
        const first = lectures[0];
        return lectures.slice(1).some((lecture) => lecture.isChatEnabled !== first.isChatEnabled);
    }

    static triggerUpdate() {
        const event = new CustomEvent("newlectures", {
            detail: {
                lectures: LectureList.lectures,
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

class TranscodingProgress {
    version: string;
    progress: number;
}

export class Lecture {
    static dateFormatOptions: Intl.DateTimeFormatOptions = {
        weekday: "long",
        year: "numeric",
        month: "short",
        day: "2-digit",
    };
    static timeFormatOptions: Intl.DateTimeFormatOptions = {
        hour: "2-digit",
        minute: "2-digit",
    };
    readonly courseId: number;
    readonly courseSlug: string;
    readonly lectureId: number;
    readonly streamKey: string;
    readonly seriesIdentifier: string;
    color: string;
    readonly vodViews: number;
    start: Date;
    end: Date;
    readonly isLiveNow: boolean;
    isConverting: boolean;
    readonly isRecording: boolean;
    readonly isPast: boolean;
    readonly hasStats: boolean;

    name: string;
    description: string;
    lectureHallId: string;
    lectureHallName: string;
    isChatEnabled = false;
    uiEditMode: UIEditMode = UIEditMode.none;
    newName: string;
    newDescription: string;
    newLectureHallId: string;
    newIsChatEnabled = false;
    isDirty = false;
    isSaving = false;
    isDeleted = false;
    lastErrors: string[] = [];
    transcodingProgresses: TranscodingProgress[];
    files: LectureFile[];
    private: boolean;
    downloadableVods: DownloadableVod[];

    clone() {
        return Object.assign(Object.create(Object.getPrototypeOf(this)), this);
    }

    startDateFormatted() {
        return this.start.toLocaleDateString("en-US", Lecture.dateFormatOptions);
    }

    startTimeFormatted() {
        return this.start.toLocaleTimeString("en-US", Lecture.timeFormatOptions);
    }

    endFormatted() {
        return this.end.toLocaleDateString("en-US", Lecture.dateFormatOptions);
    }

    endTimeFormatted() {
        return this.end.toLocaleTimeString("en-US", Lecture.timeFormatOptions);
    }

    updateIsDirty() {
        this.isDirty =
            this.newName !== this.name ||
            this.newDescription !== this.description ||
            this.newLectureHallId !== this.lectureHallId ||
            this.newIsChatEnabled !== this.isChatEnabled;
    }

    resetNewFields() {
        this.newName = this.name;
        this.newDescription = this.description;
        this.newLectureHallId = this.lectureHallId;
        this.newIsChatEnabled = this.isChatEnabled;
        this.isDirty = false;
        this.lastErrors = [];
    }

    startSeriesEdit() {
        if (this.uiEditMode !== UIEditMode.none) return;
        this.resetNewFields();
        this.uiEditMode = UIEditMode.series;
    }

    startSingleEdit() {
        if (this.uiEditMode !== UIEditMode.none) return;
        this.resetNewFields();
        this.uiEditMode = UIEditMode.single;
    }

    async toggleVisibility() {
        fetch(`/api/stream/${this.lectureId}/visibility`, {
            method: "PATCH",
            body: JSON.stringify({ private: !this.private }),
            headers: { "Content-Type": "application/json" },
        }).then((r) => {
            if (r.status == StatusCodes.OK) {
                this.private = !this.private;
                if (this.private) {
                    this.color = "gray-500";
                } else {
                    this.color = "success";
                }
            }
        });
    }

    async keepProgressesUpdated() {
        if (!this.isConverting) {
            return;
        }
        setTimeout(() => {
            for (let i = 0; i < this.transcodingProgresses.length; i++) {
                fetch(
                    `/api/course/${this.courseId}/stream/${this.lectureId}/transcodingProgress?v=${this.transcodingProgresses[i].version}`,
                )
                    .then((r) => {
                        return r.json() as Promise<number>;
                    })
                    .then((r) => {
                        if (r === 100) {
                            this.transcodingProgresses.splice(i, 1);
                        } else {
                            this.transcodingProgresses[i].progress = r;
                        }
                    });
            }
            this.isConverting = this.transcodingProgresses.length > 0;
            this.keepProgressesUpdated();
        }, 5000);
    }

    async saveEdit() {
        this.lastErrors = [];
        if (this.uiEditMode === UIEditMode.none) return;

        this.isSaving = true;
        const promises = [];
        if (this.newName !== this.name) promises.push(this.saveNewLectureName());
        if (this.newDescription !== this.description) promises.push(this.saveNewLectureDescription());
        if (this.newLectureHallId !== this.lectureHallId) promises.push(this.saveNewLectureHall());
        if (this.newIsChatEnabled !== this.isChatEnabled) promises.push(this.saveNewIsChatEnabled());

        const errors = (await Promise.all(promises)).filter((res) => res.status !== StatusCodes.OK);

        if (this.uiEditMode === UIEditMode.series && errors.length === 0) {
            const seriesUpdateResult = await this.saveSeries();
            if (seriesUpdateResult.status !== StatusCodes.OK) {
                errors.push(seriesUpdateResult);
            }
        }

        if (errors.length > 0) {
            this.lastErrors = await Promise.all(
                errors.map((e) => {
                    const text = e.text();
                    try {
                        const msg = JSON.parse(text).msg;
                        if (msg != null && msg.length > 0) {
                            return msg;
                        }
                        // eslint-disable-next-line no-empty
                    } catch (_) {}
                    return text;
                }),
            );
            this.isSaving = false;
            return false;
        }

        this.uiEditMode = UIEditMode.none;
        this.isSaving = false;
        return true;
    }

    discardEdit() {
        this.uiEditMode = UIEditMode.none;
    }

    async saveNewLectureName() {
        const res = await postData("/api/course/" + this.courseId + "/renameLecture/" + this.lectureId, {
            name: this.newName,
        });

        if (res.status == StatusCodes.OK) {
            this.name = this.newName;
        }

        return res;
    }

    async saveNewLectureDescription() {
        const res = await putData("/api/course/" + this.courseId + "/updateDescription/" + this.lectureId, {
            name: this.newDescription,
        });

        if (res.status == StatusCodes.OK) {
            this.description = this.newDescription;
        }

        return res;
    }

    async saveNewLectureHall() {
        const res = await saveLectureHall([this.lectureId], this.newLectureHallId);

        if (res.status == StatusCodes.OK) {
            this.lectureHallId = this.newLectureHallId;
            this.lectureHallName = "";
        }

        return res;
    }

    async saveNewIsChatEnabled() {
        const res = await saveIsChatEnabled(this.lectureId, this.newIsChatEnabled);

        if (res.status == StatusCodes.OK) {
            this.isChatEnabled = this.newIsChatEnabled;
        } else {
            res.text().then((t) => showMessage(t));
        }
        return res;
    }

    async saveSeries() {
        const res = await postData("/api/course/" + this.courseId + "/updateLectureSeries/" + this.lectureId);

        if (res.status == StatusCodes.OK) {
            LectureList.lectures = LectureList.lectures.map((lecture) => {
                if (this.lectureId !== lecture.lectureId && lecture.seriesIdentifier === this.seriesIdentifier) {
                    /* cloning, as otherwise alpine doesn't detect the changed object in the array ... */
                    lecture = lecture.clone();
                    lecture.name = this.name;
                    lecture.description = this.description;
                    lecture.lectureHallId = this.lectureHallId;
                    lecture.uiEditMode = UIEditMode.none;
                }
                return lecture;
            });
            LectureList.triggerUpdate();
        }

        return res;
    }

    async deleteLecture() {
        if (confirm("Confirm deleting video?")) {
            const res = await postData("/api/course/" + this.courseId + "/deleteLectures", {
                streamIDs: [this.lectureId.toString()],
            });

            if (res.status !== StatusCodes.OK) {
                alert("An unknown error occurred during the deletion process!");
                return;
            }

            LectureList.lectures = LectureList.lectures.filter((l) => l.lectureId !== this.lectureId);
            LectureList.triggerUpdate();
        }
    }

    async deleteLectureSeries() {
        const lectureCount = LectureList.lectures.filter((l) => l.seriesIdentifier === this.seriesIdentifier).length;
        if (confirm("Confirm deleting " + lectureCount + " videos in the lecture series?")) {
            const res = await Delete("/api/course/" + this.courseId + "/deleteLectureSeries/" + this.lectureId);

            if (res.status === StatusCodes.OK) {
                LectureList.lectures = LectureList.lectures.filter((l) => l.seriesIdentifier !== this.seriesIdentifier);
                LectureList.triggerUpdate();
            }

            return res;
        }
    }

    getDownloads() {
        return this.downloadableVods;
    }

    async deleteFile(fileId: number) {
        await fetch(`/api/stream/${this.lectureId}/files/${fileId}`, {
            method: "DELETE",
        })
            .catch((err) => console.log(err))
            .then(() => {
                this.files = this.files.filter((f) => f.id !== fileId);
            });
    }

    hasAttachments(): boolean {
        if (this.files !== undefined && this.files !== null) {
            const filtered = this.files.filter((f) => f.fileType === FileType.attachment);
            return filtered.length > 0;
        }
        return false;
    }

    onFileDrop(e) {
        if (e.dataTransfer.items) {
            const kind = e.dataTransfer.items[0].kind;
            switch (kind) {
                case "file": {
                    this.postFile(e.dataTransfer.items[0].getAsFile());
                    break;
                }
                case "string": {
                    this.postFileAsURL(e.dataTransfer.getData("URL"));
                }
            }
        }
    }

    private async postFile(file) {
        const formData = new FormData();
        formData.append("file", file);
        await fetch(`/api/stream/${this.lectureId}/files?type=file`, {
            method: "POST",
            body: formData,
        }).then((res) =>
            res.json().then((id) => {
                const friendlyName = file.name;
                const fileType = 2;
                this.files.push(new LectureFile({ id, fileType, friendlyName }));
            }),
        );
    }

    private async postFileAsURL(fileURL) {
        const formData = new FormData();
        formData.append("file_url", fileURL);
        await fetch(`/api/stream/${this.lectureId}/files?type=url`, {
            method: "POST",
            body: formData,
        }).then((res) =>
            res.json().then((id) => {
                const friendlyName = fileURL.substring(fileURL.lastIndexOf("/") + 1);
                const fileType = 2;
                this.files.push(new LectureFile({ id, fileType, friendlyName }));
            }),
        );
    }
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

        LectureList.lectures = LectureList.lectures.filter((l) => !lids.includes(l.lectureId));
        LectureList.triggerUpdate();
    }
}

export function saveIsChatEnabled(streamId: number, isChatEnabled: boolean) {
    return patchData("/api/stream/" + streamId + "/chat/enabled", { streamId, isChatEnabled });
}

export async function saveIsChatEnabledForAllLectures(isChatEnabled: boolean) {
    const promises = [];
    for (const lecture of LectureList.lectures) {
        promises.push(saveIsChatEnabled(lecture.lectureId, isChatEnabled));
    }
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

export function createLectureForm(args: { s: [] }) {
    return {
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
            recurring: false,
            recurringInterval: "weekly",
            eventsCount: 10,
            recurringDates: [],
            combFile: null,
            presFile: null,
            camFile: null,
        },
        streams: args.s,
        loading: false,
        error: false,
        courseID: -1,
        invalidReason: "",
        init() {
            this.onUpdate();
        },
        next() {
            this.currentTab++;
            this.onUpdate();
        },
        prev() {
            this.currentTab--;
            this.onUpdate();
        },
        updateType(vodup: boolean) {
            this.formData.vodup = vodup;
            if (vodup) {
                this.formData.recurring = false;
            }
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
        onUpdate() {
            if (this.currentTab === 0) {
                this.canContinue = true;
                this.canGoBack = false;
                this.onLastSlide = false;
                return;
            }

            if (this.currentTab === 1) {
                if (this.formData.vodup) {
                    this.canGoBack = true;
                    this.onLastSlide = false;
                    this.canContinue = this.formData.title.length > 0 && this.formData.start.length > 0;
                } else {
                    this.canGoBack = true;
                    this.onLastSlide = true;
                    this.canContinue =
                        this.formData.title.length > 0 &&
                        this.formData.start.length > 0 &&
                        this.formData.end.length > 0;
                }
                return;
            }

            if (this.currentTab === 2) {
                this.canContinue = true;
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
                        const url = new URL(window.location.href);
                        url.hash = `lectures:${ids.join(",")}`;
                        window.location.assign(url);
                        window.location.reload();
                    })
                    .catch((e) => {
                        console.log(e);
                        this.currentTab = currentTab;
                        this.loading = false;
                        this.error = true;
                    });
            }
        },

        async uploadVod(): Promise<number> {
            const uploadPres = this.formData.presFile[0] != null;
            const uploadCam = this.formData.camFile[0] != null;

            let uploadProgress = {
                COMB: 0,
                PRES: uploadPres ? 0 : null,
                CAM: uploadCam ? 0 : null,
            };

            window.dispatchEvent(new CustomEvent("voduploadprogress", { detail: uploadProgress }));

            const { streamID } = JSON.parse(
                await uploadFilePost(
                    `/api/course/${this.courseID}/uploadVOD?start=${this.formData.start}&title=${this.formData.title}`,
                    this.formData.combFile[0],
                    (progress) => {
                        uploadProgress = { ...uploadProgress, COMB: progress };
                        window.dispatchEvent(new CustomEvent("voduploadprogress", { detail: uploadProgress }));
                    },
                ),
            );

            if (uploadPres)
                await uploadFilePost(
                    `/api/course/${this.courseID}/uploadVODMedia?streamID=${streamID}&videoType=PRES`,
                    this.formData.presFile[0],
                    (progress) => {
                        uploadProgress = { ...uploadProgress, PRES: progress };
                        window.dispatchEvent(new CustomEvent("voduploadprogress", { detail: uploadProgress }));
                    },
                );

            if (uploadCam)
                await uploadFilePost(
                    `/api/course/${this.courseID}/uploadVODMedia?streamID=${streamID}&videoType=CAM`,
                    this.formData.camFile[0],
                    (progress) => {
                        uploadProgress = { ...uploadProgress, CAM: progress };
                        window.dispatchEvent(new CustomEvent("voduploadprogress", { detail: uploadProgress }));
                    },
                );

            return streamID;
        },
    };
}

function uploadFilePost(url: string, file: File, onProgress: (progress: number) => void): Promise<string> {
    const xhr = new XMLHttpRequest();
    const vodUploadFormData = new FormData();
    vodUploadFormData.append("file", file);
    return new Promise((resolve, reject) => {
        xhr.onloadend = () => {
            if (xhr.status === 200) {
                resolve(xhr.responseText);
            } else {
                reject();
            }
        };
        xhr.upload.onprogress = (e: ProgressEvent) => {
            if (!e.lengthComputable) {
                return;
            }
            onProgress(Math.floor(100 * (e.loaded / e.total)));
        };
        xhr.open("POST", url);
        xhr.send(vodUploadFormData);
    });
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
