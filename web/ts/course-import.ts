const d = {step: 0, year: 2021, semester: "W", department: "In", optInOut:"Opt In", loading: false, range: "", courses: []};

export function pageData() {
    return d;
}

// lecture hall selected -> api call
export function addNotifyEventListeners() {
    window.addEventListener("notify1", () => {
        // warn users when leaving site:
        window.addEventListener("beforeunload", function (e) {
            e.preventDefault(); // If you prevent default behavior in Mozilla Firefox prompt will always be shown
            // Chrome requires returnValue to be set
            e.returnValue = "";
        });
        window.dispatchEvent(new CustomEvent("loading-start"));
        fetch(`/api/course-schedule?range=${d.range}&department=${d.department}`).then((res) => {
            res.text().then((text) => {
                console.log(text);
                window.dispatchEvent(new CustomEvent("loading-end", {detail: {courses: JSON.parse(text)}}));
            });
        });
    });
    window.addEventListener("notify2", () => {
        fetch(`/api/course-schedule/${d.year}/${d.semester}`, {
            method: "POST",
            body: JSON.stringify({courses: d.courses, optIn: d.optInOut === "Opt In"}),
        }).then((r) => window.dispatchEvent(new CustomEvent("imported", {detail: r.status})));
    });

    window.addEventListener("notify3", () => {
        window.location.replace("/");
    });
}
