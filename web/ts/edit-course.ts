class EditCourse {
    private start
    private end

    constructor() {
        let createBtn = document.getElementById("createLectureBtn");
        if (createBtn !== null && createBtn !== undefined) {
            createBtn.addEventListener("click", (e: Event) => this.createLecture())
            // @ts-ignore
            this.start = flatpickr("#start", {enableTime: true, time_24hr: true});
            // @ts-ignore
            this.end = flatpickr("#end", {enableTime: true, time_24hr: true});
        }
    }

    private createLecture(): void {
        const id = (document.getElementById("courseID") as HTMLInputElement).value
        const name = (document.getElementById("name") as HTMLInputElement).value
        postData("/api/course/" + id + "/createLecture", {
            "name": name,
            "start": this.start.selectedDates[0].toISOString(),
            "end": this.end.selectedDates[0].toISOString(),
        }).then(data => {
            if (data.status != 200) {
                data.text().then(t => showMessage(t))
            } else {
                window.location.reload()
            }
        })
    }
}

function saveLectureHall(lectureID: number) {
    postData("/api/updateLecturesLectureHall", {
        "lecture": lectureID,
        "lectureHall": parseInt((document.getElementById("lectureHallSelector" + lectureID) as HTMLSelectElement).selectedOptions[0].value)
    }).then(res => {
        if (res.status === 200) {
            document.getElementById("applyLectureHall" + lectureID).classList.add("hidden")
        }
    })
}

function saveLectureDescription(e: Event, cID: number, lID: number) {
    e.preventDefault()
    const input = (document.getElementById("lectureDescriptionInput" + lID) as HTMLInputElement).value
    postData("/api/course/" + cID + "/updateDescription/" + lID, {"name": input})
        .then(res => {
            if (res.status == 200) {
                document.getElementById("descriptionSubmitBtn" + lID).classList.add("invisible")
            } else {
                res.text().then(t => showMessage(t))
            }
        })
}

function saveLectureName(e: Event, cID: number, lID: number) {
    e.preventDefault()
    const input = (document.getElementById("lectureNameInput" + lID) as HTMLInputElement).value
    postData("/api/course/" + cID + "/renameLecture/" + lID, {"name": input})
        .then(res => {
            if (res.status == 200) {
                document.getElementById("nameSubmitBtn" + lID).classList.add("invisible")
            } else {
                res.text().then(t => showMessage(t))
            }
        })
}

function showStats(id: number): void {
    if (document.getElementById("statsBox" + id).classList.contains("hidden")) {
        document.getElementById("statsBox" + id).classList.remove("hidden")
    } else {
        document.getElementById("statsBox" + id).classList.add("hidden")
    }
}

function focusNameInput(input: HTMLInputElement, id: number) {
    input.oninput = function () {
        document.getElementById("nameSubmitBtn" + id).classList.remove("invisible")
    }
}

function focusDescriptionInput(input: HTMLInputElement, id: number) {
    input.oninput = function () {
        document.getElementById("descriptionSubmitBtn" + id).classList.remove("invisible")
    }
}

function toggleExtraInfos(btn: HTMLElement, id: number) {
    btn.classList.add("transform", "transition", "duration-500", "ease-in-out")
    if (btn.classList.contains("rotate-180")) {
        btn.classList.remove("rotate-180")
        document.getElementById("extraInfos" + id).classList.add("hidden")
    } else {
        btn.classList.add("rotate-180")
        document.getElementById("extraInfos" + id).classList.remove("hidden")
    }
}

function deleteLecture(cid: number, lid: number) {
    if (confirm("Confirm deleting video?")) {
        postData("/api/course/" + cid + "/deleteLecture/" + lid).then(r => {
            document.location.reload()
        })
    }
}

function showHideUnits(id: number) {
    const container = document.getElementById('unitsContainer' + id)
    if (container.classList.contains("hidden")) {
        container.classList.remove("hidden")
    } else {
        container.classList.add("hidden")
    }
}

function addUnit(streamID: number): boolean {
    return false
}

window.onload = function () {
    new EditCourse()
}