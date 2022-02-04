import { loadStats } from "./stats";
import { postData, showMessage } from "./global";
import { StatusCodes } from "http-status-codes";

/**
 * loadGeneralStats gets the audience of the course from the api (live and vod) and renders it into a graph.
 */
export function loadGeneralStats() {
    loadStats("activity", "courseGeneralStatsLive");
}

export function saveLectureHall(streamIds: number[], lectureHall: string) {
    return postData("/api/setLectureHall", { streamIds, lectureHall: parseInt(lectureHall) });
}

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

export function focusNameInput(input: HTMLInputElement, id: number) {
    input.oninput = function () {
        document.getElementById("nameSubmitBtn" + id).classList.remove("invisible");
    };
}

export function focusDescriptionInput(input: HTMLInputElement, id: number) {
    input.oninput = function () {
        document.getElementById("descriptionSubmitBtn" + id).classList.remove("invisible");
    };
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

export function deleteLecture(cid: number, lid: number) {
    if (confirm("Confirm deleting video?")) {
        postData("/api/course/" + cid + "/deleteLectures", { streamIDs: [lid.toString()] }).then(() => {
            document.location.reload();
        });
    }
}

export async function deleteLectures(cid: number, lids: number[]) {
    if (confirm("Confirm deleting " + lids.length + " video" + (lids.length == 1 ? "" : "s") + "?")) {
        await postData("/api/course/" + cid + "/deleteLectures", { streamIDs: lids.map((n) => n.toString()) });
        document.location.reload();
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
            recurringCount: 10,
            recurringDates: [],
            file: null,
        },
        loading: false,
        error: false,
        courseID: -1,
        regenerateRecurringDates() {
            const result = [];
            if (this.formData.start != "") {
                for (let i = 0; i < this.formData.recurringCount; i++) {
                    let date;
                    if (i == 0) {
                        date = new Date(this.formData.start);
                    } else {
                        date = new Date(result[i - 1].date);
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
            const body = new FormData();
            body.set("title", this.formData.title);
            body.set("lectureHallId", this.formData.lectureHallId);
            body.set("premiere", this.formData.premiere);
            body.set("vodup", this.formData.vodup);
            body.set("start", this.formData.start);
            body.set("duration", this.formData.duration);
            if (this.formData.recurring) {
                for (const date of this.formData.recurringDates.filter(({ enabled }) => enabled)) {
                    body.append("dateSeries[]", date.date.toISOString());
                }
            }
            if (this.formData.premiere || this.formData.vodup) {
                body.set("file", this.formData.file[0]);
                body.set("duration", "0"); // premieres have no explicit end set -> use "0" here
            }
            fetch("/api/course/" + this.courseID + "/createLecture", {
                method: "POST",
                body: body,
            })
                .then(() => {
                    this.loading = false;
                    window.location.reload();
                })
                .catch(() => {
                    this.loading = false;
                    this.error = true;
                });
        },
    };
}
