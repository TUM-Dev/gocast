import { StatusCodes } from "http-status-codes";
import { postData, showMessage } from "./global";

export class CreateCourse {
    private courseIDInput: HTMLInputElement;
    private loadFromTUMOnlineBtn: HTMLButtonElement;
    private courseNameInput: HTMLInputElement;
    private semesterInput: HTMLInputElement;
    private yearInput: HTMLInputElement;
    private yearWWInput: HTMLInputElement; // for the '/21' part
    private slugInput: HTMLInputElement;
    private tumOnlineInfo: HTMLSpanElement;
    private enrolledRadio: HTMLInputElement;

    constructor() {
        this.loadFromTUMOnlineBtn = document.getElementById("loadCourseInfoBtn") as HTMLButtonElement;
        this.loadFromTUMOnlineBtn.addEventListener("click", () => this.loadCourseInfo());
        this.courseIDInput = document.getElementById("courseID") as HTMLInputElement;
        this.courseNameInput = document.getElementById("name") as HTMLInputElement;
        this.semesterInput = document.getElementById("semester") as HTMLInputElement;
        this.yearInput = document.getElementById("year") as HTMLInputElement;
        this.yearWWInput = document.getElementById("yearW") as HTMLInputElement;
        this.slugInput = document.getElementById("slug") as HTMLInputElement;
        this.tumOnlineInfo = document.getElementById("TUMOnlineInfo") as HTMLSpanElement;
        this.enrolledRadio = document.getElementById("enrolled") as HTMLInputElement;
        document.getElementById("createCourseBtn")?.addEventListener("click", (e: Event) => {
            e.preventDefault();
            this.createCourse();
            return false;
        });
    }

    private loadCourseInfo(): void {
        if (this.loadFromTUMOnlineBtn.disabled) {
            return;
        }
        postData("/api/courseInfo", { courseID: this.courseIDInput.value }).then((data) => {
            if (data.status != StatusCodes.OK) {
                showMessage(
                    "The course with this ID was not found in TUMOnline. Please verify the ID or reach out to us.",
                );
            } else {
                data.text().then((data) => {
                    const json = JSON.parse(data);
                    const teachingTerm = json["teachingTerm"].Split(" ");
                    this.courseNameInput.value = json["courseName"];
                    this.semesterInput.value = teachingTerm[0];
                    this.yearInput.value = teachingTerm[1];
                    this.tumOnlineInfo.innerText =
                        "Currently there are " +
                        json["numberAttendees"] +
                        " students enrolled in this course. Please verify that this looks right.";
                    this.enrolledRadio.removeAttribute("disabled");
                });
            }
        });
    }

    private createCourse(): void {
        const f = new FormData(document.getElementById("createCourseForm") as HTMLFormElement);
        let teachingTerm;
        if (f.get("semester") === "Wintersemester") {
            teachingTerm = `${f.get("semester")} ${f.get("year")}/${f.get("yearW")}`;
        } else {
            teachingTerm = `${f.get("semester")} ${f.get("year")}`;
        }
        postData("/api/createCourse", {
            courseID: f.get("courseID"),
            name: f.get("name"),
            teachingTerm: teachingTerm,
            slug: f.get("slug"),
            access: f.get("access"),
            enVOD: f.get("enVOD") === "on",
            enDL: f.get("enDL") === "on",
            enChat: f.get("enChat") === "on",
        }).then((data) => {
            if (data.status != StatusCodes.OK) {
                data.text().then((t) => showMessage(t));
            } else {
                window.location.href = "/admin";
            }
        });
    }
}
