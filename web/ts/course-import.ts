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

/**
 * onBeforeUnloadHandle is a function that will create a prompt asking the user if they want to leave
 *
 * The reason this has been made a separate Function and not in the "addEventListener" is because
 * the function has to be taken out in the EventListener so that after the import has been done, the
 * user does not need to be asked if they wanted to leave
 * @param e is just the event that it will accept from the BeforeUnload event
 */

function onBeforeUnloadHandle(e) {
    e.preventDefault(); // If you prevent default behavior in Mozilla Firefox, prompt will always be shown
    e.returnValue = ""; //Chrome requires returnValue to be set
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

        fetch(`/api/course-schedule/${d.year}/${d.semester}`, {
            method: "POST",
            body: JSON.stringify({ courses: d.courses, optIn: d.optInOut === "Opt In" }),
        }).then((r) => window.dispatchEvent(new CustomEvent("imported", { detail: r.status })));

        //Removes the eventListener after import has been done since user won't change anything no longer
        window.removeEventListener("beforeunload", onBeforeUnloadHandle);
    });

    window.addEventListener("notify3", () => {
        window.location.replace("/");
    });
}
