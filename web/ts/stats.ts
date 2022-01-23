import { StatusCodes } from "http-status-codes";
import Chart from "chart.js/auto";

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
