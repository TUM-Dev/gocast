import {StatusCodes} from "http-status-codes";

export module Stats {
    export function loadStats(endpoint: string, targetEl: string) {
        getAsync(`/api/course/${(document.getElementById("courseID") as HTMLInputElement).value}/stats?interval=${endpoint}`).then(res => {
                if (res.status === StatusCodes.OK) {
                    res.text().then(value => {
                        // @ts-ignore
                        new Chart(
                            document.getElementById(targetEl), JSON.parse(value),
                        );
                    });
                }
            }
        );
    }

    export function initStatsPage() {
        let dates = ["numStudents", "vodViews", "liveViews"];
        dates.forEach(endpoint => {
            getAsync(`/api/course/${(document.getElementById("courseID") as HTMLInputElement).value}/stats?interval=${endpoint}`).then(res => {
                    if (res.status === StatusCodes.OK) {
                        res.text().then(value => {
                            document.getElementById(endpoint).innerHTML = `<span>${JSON.parse(value)["res"]}</span>`
                        });
                    }
                }
            );
        });
    }

    export async function getAsync(url = '') {
        return await fetch(url, {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json'
            },
        });
    }
}
