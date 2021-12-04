function loadStats(endpoint: string, targetEl: string) {
    const canvas = <HTMLCanvasElement>document.getElementById(targetEl);
    const ctx = canvas.getContext("2d");
    getAsync(
        `/api/course/${(document.getElementById("courseID") as HTMLInputElement).value}/stats?interval=${endpoint}`,
    ).then((res) => {
        if (res.status === 200) {
            res.text().then((value) => {
                // @ts-ignore
                new Chart(ctx, JSON.parse(value));
            });
        }
    });
}

function initStatsPage() {
    const dates = ["numStudents", "vodViews", "liveViews"];
    dates.forEach((endpoint) => {
        getAsync(
            `/api/course/${(document.getElementById("courseID") as HTMLInputElement).value}/stats?interval=${endpoint}`,
        ).then((res) => {
            if (res.status === 200) {
                res.text().then((value) => {
                    document.getElementById(endpoint).innerHTML = `<span>${JSON.parse(value)["res"]}</span>`;
                });
            }
        });
    });
}

async function getAsync(url = "") {
    return await fetch(url, {
        method: "GET",
        headers: {
            "Content-Type": "application/json",
        },
    });
}
