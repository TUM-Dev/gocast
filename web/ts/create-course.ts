import { StatusCodes } from "http-status-codes";
import { postData, showMessage } from "./global";

export function loadCourseInfo(
    id: string,
): Promise<{ title: string; semester: string; year: number; yearW: number; numberAttendees: number } | any> {
    return fetch("/api/courseInfo", { method: "POST", body: JSON.stringify({ courseID: id }) }).then((data) => {
        if (data.status != StatusCodes.OK) {
            showMessage("The course with this ID was not found in TUMOnline. Please verify the ID or reach out to us.");
            return null;
        } else {
            return data.json().then((data) => {
                const years = data["teachingTerm"].split(" ")[1].split("/");
                return {
                    title: data["courseName"],
                    semester: data["teachingTerm"].split(" ")[0],
                    year: parseInt(years[0]),
                    yearW: parseInt(years[1]),
                    numberAttendees: data["numberAttendees"],
                };
            });
        }
    });
}

export function createCourse(
    id: string,
    semester: string,
    year: number,
    yearW: number,
    name: string,
    slug: string,
): void {
    let teachingTerm;
    if (semester === "Wintersemester") {
        teachingTerm = `${semester} ${year}/${yearW}`;
    } else {
        teachingTerm = `${semester} ${year}`;
    }
    postData("/api/createCourse", {
        courseID: id,
        name: name,
        teachingTerm: teachingTerm,
        slug: slug,
        // defaults:
        access: "loggedin",
        enVOD: true,
        enDL: false,
        enChat: false,
    }).then((data) => {
        if (data.status !== StatusCodes.CREATED) {
            data.text().then((t) => showMessage(t));
        } else {
            data.json().then((resp) => {
                window.location.href = "/admin/course/" + resp["id"] + "?created";
            });
        }
    });
}
