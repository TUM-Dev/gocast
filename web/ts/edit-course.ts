import { Delete, postData, putData, showMessage } from "./global";
import { StatusCodes } from "http-status-codes";

export enum UIEditMode {
    none,
    single,
    series,
}

export class LectureList {
    static lectures: Lecture[] = [];

    static init(initialState, triggerUpdateFunc) {
        LectureList.lectures = initialState;
        LectureList.triggerUpdate();
    }

    static triggerUpdate() {
        const event = new CustomEvent("newlectures", { detail: LectureList.lectures });
        window.dispatchEvent(event);
    }
}

class LectureFile {
    readonly id: number;
    readonly friendlyName: string;

    constructor({ id, friendlyName }) {
        this.id = id;
        this.friendlyName = friendlyName;
    }
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
    readonly color: string;
    readonly vodViews: number;
    readonly start: Date;
    readonly end: Date;
    readonly isLiveNow: boolean;
    readonly isRecording: boolean;
    readonly isPast: boolean;
    readonly hasStats: boolean;
    readonly files: LectureFile[];

    name: string;
    description: string;
    lectureHallId: string;
    lectureHallName: string;
    uiEditMode: UIEditMode = UIEditMode.none;
    newName: string;
    newDescription: string;
    newLectureHallId: string;
    isDirty = false;
    isSaving = false;
    isDeleted = false;
    lastErrors: string[] = [];

    constructor(
        {
            courseId,
            lectureId,
            seriesIdentifier,
            name,
            description,
            lectureHallId,
            lectureHallName,
            streamKey,
            isPast,
            isLiveNow,
            isRecording,
            files,
            hasStats,
            color,
            start,
            end,
        },
        courseSlug: string,
    ) {
        this.courseId = courseId;
        this.lectureId = lectureId;
        this.seriesIdentifier = seriesIdentifier;
        this.lectureHallId = "" + lectureHallId;
        this.lectureHallName = lectureHallName;
        this.name = name;
        this.description = description;
        this.streamKey = streamKey;
        this.courseSlug = courseSlug;
        this.isPast = isPast;
        this.isLiveNow = isLiveNow;
        this.isRecording = isRecording;
        this.hasStats = hasStats;
        this.color = color;
        this.start = new Date(start);
        this.end = new Date(end);
        this.files = files === null ? [] : files.map((file) => new LectureFile(file));
    }

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
            this.newLectureHallId !== this.lectureHallId;
    }

    resetNewFields() {
        this.newName = this.name;
        this.newDescription = this.description;
        this.newLectureHallId = this.lectureHallId;
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

    async saveEdit() {
        this.lastErrors = [];
        if (this.uiEditMode === UIEditMode.none) return;

        this.isSaving = true;
        const promises = [];
        if (this.newName !== this.name) promises.push(this.saveNewLectureName());
        if (this.newDescription !== this.description) promises.push(this.saveNewLectureDescription());
        if (this.newLectureHallId !== this.lectureHallId) promises.push(this.saveNewLectureHall());

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

export function saveLectureHall(streamIds: number[], lectureHall: string) {
    return postData("/api/setLectureHall", { streamIds, lectureHall: parseInt(lectureHall) });
}

// Used by schedule.ts
export function saveLectureDescription(e: Event, cID: number, lID: number) {
    e.preventDefault();
    const input = (document.getElementById("lectureDescriptionInput" + lID) as HTMLInputElement).value;
    postData("/api/course/" + cID + "/updateDescription/" + lID, { name: input }).then((res) => {
        if (res.status == StatusCodes.OK) {
            document.getElementById("descriptionSubmitBtn" + lID).classList.add("invisible");
        } else {
            res.text().then((t) => showMessage(t));
        }
    });
}

// Used by schedule.ts
export function saveLectureName(e: Event, cID: number, lID: number) {
    e.preventDefault();
    const input = (document.getElementById("lectureNameInput" + lID) as HTMLInputElement).value;
    postData("/api/course/" + cID + "/renameLecture/" + lID, { name: input }).then((res) => {
        if (res.status == StatusCodes.OK) {
            document.getElementById("nameSubmitBtn" + lID).classList.add("invisible");
        } else {
            res.text().then((t) => showMessage(t));
        }
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

export function createLectureForm() {
    return {
        formData: {
            title: "",
            lectureHallId: 0,
            start: "",
            end: "",
            duration: 0, // Duration in Minutes
            formatedDuration: "", // Duration in Minutes
            premiere: false,
            vodup: false,
            recurring: false,
            recurringInterval: "weekly",
            eventsCount: 10,
            recurringDates: [],
            file: null,
        },
        loading: false,
        error: false,
        courseID: -1,
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
        submitData() {
            this.loading = true;
            if (this.formData.vodup) {
                this.uploadVod();
            } else {
                const payload = {
                    title: this.formData.title,
                    lectureHallId: this.formData.lectureHallId.toString(),
                    premiere: this.formData.premiere,
                    vodup: this.formData.vodup,
                    start: this.formData.start,
                    duration: this.formData.duration,
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
                    .then(() => {
                        this.loading = false;
                        window.location.reload();
                    })
                    .catch(() => {
                        this.loading = false;
                        this.error = true;
                    });
            }
        },
        uploadVod() {
            const xhr = new XMLHttpRequest();
            const vodUploadFormData = new FormData();
            vodUploadFormData.append("file", this.formData.file[0]);
            xhr.onloadend = () => {
                if (xhr.status === 200) {
                    window.location.reload();
                } else {
                    this.error = true;
                }
            };
            xhr.upload.onprogress = (e: ProgressEvent) => {
                if (!e.lengthComputable) {
                    return;
                }
                window.dispatchEvent(
                    new CustomEvent("voduploadprogress", { detail: Math.floor(100 * (e.loaded / e.total)) }),
                );
            };
            xhr.open("POST", `/api/course/${this.courseID}/uploadVOD?start=${this.formData.start}`);
            xhr.send(vodUploadFormData);
        },
    };
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
