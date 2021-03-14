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

new EditCourse()
