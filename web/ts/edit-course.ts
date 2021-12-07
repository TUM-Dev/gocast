class EditCourse {
    constructor() {
        EditCourse.loadGeneralStats();
    }

    /**
     * loadGeneralStats gets the audience of the course from the api (live and vod) and renders it into a graph
     * @private
     */
    private static loadGeneralStats() {
        loadStats("activity", "courseGeneralStatsLive");
    }
}

function saveLectureHall(lectureID: number) {
    postData("/api/updateLecturesLectureHall", {
        lecture: lectureID,
        lectureHall: parseInt(
            (document.getElementById("lectureHallSelector" + lectureID) as HTMLSelectElement).selectedOptions[0].value,
        ),
    }).then((res) => {
        if (res.status === 200) {
            document.getElementById("applyLectureHall" + lectureID).classList.add("hidden");
        }
    });
}

function saveLectureDescription(e: Event, cID: number, lID: number) {
    e.preventDefault();
    const input = (document.getElementById("lectureDescriptionInput" + lID) as HTMLInputElement).value;
    postData("/api/course/" + cID + "/updateDescription/" + lID, { name: input }).then((res) => {
        if (res.status == 200) {
            document.getElementById("descriptionSubmitBtn" + lID).classList.add("invisible");
        } else {
            res.text().then((t) => showMessage(t));
        }
    });
}

function saveLectureName(e: Event, cID: number, lID: number) {
    e.preventDefault();
    const input = (document.getElementById("lectureNameInput" + lID) as HTMLInputElement).value;
    postData("/api/course/" + cID + "/renameLecture/" + lID, { name: input }).then((res) => {
        if (res.status == 200) {
            document.getElementById("nameSubmitBtn" + lID).classList.add("invisible");
        } else {
            res.text().then((t) => showMessage(t));
        }
    });
}

function showStats(id: number): void {
    if (document.getElementById("statsBox" + id).classList.contains("hidden")) {
        document.getElementById("statsBox" + id).classList.remove("hidden");
    } else {
        document.getElementById("statsBox" + id).classList.add("hidden");
    }
}

function focusNameInput(input: HTMLInputElement, id: number) {
    input.oninput = function () {
        document.getElementById("nameSubmitBtn" + id).classList.remove("invisible");
    };
}

function focusDescriptionInput(input: HTMLInputElement, id: number) {
    input.oninput = function () {
        document.getElementById("descriptionSubmitBtn" + id).classList.remove("invisible");
    };
}

function toggleExtraInfos(btn: HTMLElement, id: number) {
    btn.classList.add("transform", "transition", "duration-500", "ease-in-out");
    if (btn.classList.contains("rotate-180")) {
        btn.classList.remove("rotate-180");
        document.getElementById("extraInfos" + id).classList.add("hidden");
    } else {
        btn.classList.add("rotate-180");
        document.getElementById("extraInfos" + id).classList.remove("hidden");
    }
}

function deleteLecture(cid: number, lid: number) {
    if (confirm("Confirm deleting video?")) {
        postData("/api/course/" + cid + "/deleteLecture/" + lid).then(() => {
            document.location.reload();
        });
    }
}

function showHideUnits(id: number) {
    const container = document.getElementById("unitsContainer" + id);
    if (container.classList.contains("hidden")) {
        container.classList.remove("hidden");
    } else {
        container.classList.add("hidden");
    }
}

function createLectureForm() {
    return {
        formData: {
            title: "",
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

window.onload = function () {
    new EditCourse();
};
