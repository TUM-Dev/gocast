class EditCourse {
    constructor() {
        document.getElementById("createLectureBtn").addEventListener("click", (e: Event) => EditCourse.createLecture())
    }

    private static createLecture(): void {
        const id = (document.getElementById("courseID") as HTMLInputElement).value
        const name = (document.getElementById("name") as HTMLInputElement).value
        const date = (document.getElementById("date") as HTMLInputElement).value.split("-").map(value => parseInt(value))
        const time = (document.getElementById("time") as HTMLInputElement).value.split(":").map(value => parseInt(value))
        const datetime = new Date()
        datetime.setFullYear(date[0], date[1] - 1, date[2]) // wtf js??? month is 0 based, year and day 1 based.
        datetime.setHours(time[0], time[1], 0, 0)
        postData("/api/createLecture", {
            "id": id,
            "name": name,
            "start": datetime.toISOString(),
        }).then(data => {
            if (data.status != 200) {
                data.text().then(t => showMessage(t))
            } else {
                window.location.reload()
            }
        })
    }
}

function saveLectureName(e: Event, id: number) {
    e.preventDefault()
    const input = (document.getElementById("lectureNameInput" + id) as HTMLInputElement).value
    postData("/api/renameLecture", {"id": id, "name": input})
        .then(res => {
            if (res.status == 200) {
                document.getElementById("nameSubmitBtn" + id).classList.add("invisible")
            } else {
                res.text().then(t => showMessage(t))
            }
        })
}

function cutVod(id: number): void {
    document.getElementById("slider" + id).classList.remove("hidden")
    const slider = document.getElementById('slider' + id);

    // @ts-ignore
    noUiSlider.create(slider, {
        start: [0, 100],
        connect: true,
        range: {
            'min': 0,
            'max': 100
        }
    });
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

new EditCourse()
