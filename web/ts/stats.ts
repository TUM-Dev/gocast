import { StatusCodes } from "http-status-codes";
import Chart from "chart.js/auto";

export function downloadStats(format: string) {
    const statsToExport = ["week", "hour", "activity-live", "activity-vod", "allDays", "quickStats"];
    getAsync(
        `/api/course/${
            (document.getElementById("courseID") as HTMLInputElement).value
        }/stats/export?interval[]=${statsToExport.join("&interval[]=")}&format=${format}`,
    ).then(async (res) => {
        if (res.status === StatusCodes.OK) {
            const objectUrl = window.URL.createObjectURL(await res.blob());

            const anchor = document.createElement("a");
            document.body.appendChild(anchor);
            anchor.href = objectUrl;
            anchor.download = `course-${
                (document.getElementById("courseID") as HTMLInputElement).value
            }-stats.${format}`;
            anchor.click();
            document.body.removeChild(anchor);
            window.URL.revokeObjectURL(objectUrl);
        } else {
            alert("Something went wrong during export. Error-Code: " + res.status);
        }
    });
}

export function loadStats(endpoint: string, targetEl: string) {
    const canvas = <HTMLCanvasElement>document.getElementById(targetEl);
    const ctx = canvas.getContext("2d");
    getAsync(
        `/api/course/${(document.getElementById("courseID") as HTMLInputElement).value}/stats?interval=${endpoint}`,
    ).then((res) => {
        if (res.status === StatusCodes.OK) {
            res.text().then((value) => {
                new Chart(ctx, JSON.parse(value));
            });
        }
    });
}

export function initStatsPage() {
    const dates = ["numStudents", "vodViews", "liveViews"];
    dates.forEach((endpoint) => {
        getAsync(
            `/api/course/${(document.getElementById("courseID") as HTMLInputElement).value}/stats?interval=${endpoint}`,
        ).then((res) => {
            if (res.status === StatusCodes.OK) {
                res.text().then((value) => {
                    document.getElementById(endpoint).innerHTML = `<span>${JSON.parse(value)["res"]}</span>`;
                });
            }
        });
    });
}

export async function getAsync(url = "") {
    return await fetch(url, {
        method: "GET",
        headers: {
            "Content-Type": "application/json",
        },
    });
}
