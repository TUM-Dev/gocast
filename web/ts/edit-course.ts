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
            premiere: false,
            vodup: false,
            file: null,
        },
        loading: false,
        error: false,
        courseID: -1,
        submitData() {
            this.loading = true;
            console.log(this.formData);
            const body = new FormData();
            body.set("title", this.formData.title);
            body.set("lectureHallId", this.formData.lectureHallId);
            body.set("premiere", this.formData.premiere);
            body.set("vodup", this.formData.vodup);
            body.set("start", this.formData.start);
            body.set("end", this.formData.end);
            if (this.formData.premiere || this.formData.vodup) {
                body.set("file", this.formData.file[0]);
                body.set("end", this.formData.start); // premieres have no explicit end set -> use start here
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
