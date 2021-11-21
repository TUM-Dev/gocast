const d = {'step': 0, 'year': 2021, 'semester': 'W', 'loading': false, 'range': '', 'courses': []};

export module CourseImport {
    export function pageData() {
        return d;
    }

    // lecture hall selected -> api call
    window.addEventListener("notify1", () => {
        // warn users when leaving site:
        window.addEventListener("beforeunload", function (e) {
            e.preventDefault(); // If you prevent default behavior in Mozilla Firefox prompt will always be shown
            // Chrome requires returnValue to be set
            e.returnValue = '';
        });
        window.dispatchEvent(new CustomEvent("loading-start"));
        fetch(`/api/course-schedule?range=${d.range}`).then(res => {
            res.text().then(text => {
                console.log(text);
                window.dispatchEvent(new CustomEvent("loading-end", {detail: {'courses': JSON.parse(text)}}));
            });
        });
    });

    window.addEventListener("notify2", () => {
        fetch(`/api/course-schedule/${d.year}/${d.semester}`, {
            method: "POST",
            body: JSON.stringify(d.courses)
        }).then(r => window.dispatchEvent(new CustomEvent("imported", {detail: r.status})));
    });

    window.addEventListener("notify3", () => {
        window.location.replace("/");
    });
}