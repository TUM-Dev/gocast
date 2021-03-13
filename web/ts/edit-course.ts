class EditCourse {
    constructor() {
        document.getElementById("createLectureBtn").addEventListener("click", (e: Event) => EditCourse.createLecture())
    }

    private static createLecture(): void {
        const id = (document.getElementById("courseID") as HTMLInputElement).value
        const name = (document.getElementById("name") as HTMLInputElement).value
        const date = (document.getElementById("date") as HTMLInputElement).value
        const time = (document.getElementById("time") as HTMLInputElement).value
        postData("/api/createLecture", {
            "id": id,
            "name": name,
            "date": date,
            "time": time,
        }).then(data=>{
            if (data.status!=200){
                data.text().then(t=>showMessage(t))
            }else {
                data.text().then(t=>console.log(t))
            }
        })
    }
}

new EditCourse()
