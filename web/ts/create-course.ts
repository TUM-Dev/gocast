class CreateCourse {
    private courseIDInput: HTMLInputElement
    private loadFromTUMOnlineBtn: HTMLDivElement
    private courseNameInput: HTMLInputElement
    private teachingTermInput: HTMLInputElement;
    private slugInput: HTMLInputElement;
    private TUMOnlineInfo: HTMLSpanElement;
    private EnrolledRadio: HTMLInputElement;

    constructor() {
        this.loadFromTUMOnlineBtn = document.getElementById("loadCourseInfoBtn") as HTMLDivElement
        this.loadFromTUMOnlineBtn.addEventListener("click", (e: Event) => this.loadCourseInfo())
        this.courseIDInput = document.getElementById("courseID") as HTMLInputElement
        this.courseNameInput = document.getElementById("name") as HTMLInputElement
        this.teachingTermInput = document.getElementById("teachingTerm") as HTMLInputElement
        this.slugInput = document.getElementById("slug") as HTMLInputElement
        this.TUMOnlineInfo = document.getElementById("TUMOnlineInfo") as HTMLSpanElement
        this.EnrolledRadio = document.getElementById("enrolled") as HTMLInputElement
    }

    private loadCourseInfo(): void {
        postData("/api/courseInfo", {"courseID": this.courseIDInput.value})
            .then(data => {
                if (data.status != 200) {
                    this.TUMOnlineInfo.innerText = "The course with this ID was not found in TUMOnline. Please verify the ID or reach out to us."
                } else {
                    data.text().then(data => {
                        const json = JSON.parse(data)
                        this.courseNameInput.value = json["courseName"]
                        this.teachingTermInput.value = json["teachingTerm"]
                        this.TUMOnlineInfo.innerText = "Currently there are " + json["numberAttendees"] + " students enrolled in this course. Please verify that this looks right."
                        this.EnrolledRadio.removeAttribute("disabled")
                    })
                }
            })
    }
}

new CreateCourse()
