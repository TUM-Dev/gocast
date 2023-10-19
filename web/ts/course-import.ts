const d = {
    step: 0,
    year: 2022,
    semester: "W",
    department: "Computer Science",
    departmentID: "",
    optInOut: "Opt In",
    loading: false,
    range: "",
    courses: [],
};

export function pageData() {
    return d;
}

// basic function that warns users when leaving without changes
function onBeforeUnloadHandle(e) {
    e.preventDefault();
    e.returnValue = "";
}

// lecture hall selected -> api call
export function addNotifyEventListeners() {
    window.addEventListener("notify1", () => {
        // warn users when leaving site using the given function:
        window.addEventListener("beforeunload", onBeforeUnloadHandle);

        window.dispatchEvent(new CustomEvent("loading-start"));
        fetch(`/api/course-schedule?range=${d.range}&department=${d.department}&departmentID=${d.departmentID}`).then(
            (res) => {
                res.text().then((text) => {
                    console.log(text);
                    window.dispatchEvent(new CustomEvent("loading-end", { detail: { courses: JSON.parse(text) } }));
                });
            },
        );
    });

    window.addEventListener("notify2", () => {
        //Removes the eventListener after import has been done since user won't change anything no longer
        window.removeEventListener("beforeunload", onBeforeUnloadHandle);

        fetch(`/api/course-schedule/${d.year}/${d.semester}`, {
            method: "POST",
            body: JSON.stringify({ courses: d.courses, optIn: d.optInOut === "Opt In" }),
        }).then((r) => window.dispatchEvent(new CustomEvent("imported", { detail: r.status })));
    });

    window.addEventListener("notify3", () => {
        window.location.replace("/");
    });
}
