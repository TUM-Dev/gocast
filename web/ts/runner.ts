/** Make DELETE call to /api/runner/:hostname with given runner-hostname */
export async function deleteRunner(hostname: string) {
    return await fetch("/api/runners/" + hostname, {
        method: "DELETE",
    });
}

const r = {
    showFailed: false,
    showActions: false,
    failedActions: [],
    actions: [],
};

export function runnerData() {
    return r;
}

export function getFailedAction(): void {
    window.dispatchEvent(new CustomEvent("load-failures"));
    fetch("/api/Actions/failed").then((res) => {
        res.text().then((text) => {
            console.log(text);
            window.dispatchEvent(
                new CustomEvent("FailedActionListing", {
                    detail: {
                        failedActions: JSON.parse(text),
                    },
                }),
            );
        });
    });
    r
}

export function listActions(actions: string): void {
    window.dispatchEvent(new CustomEvent("load-actions"));
    actions.split(",\n").forEach((id) => {
            if (id === "") {
                return;
            }
            fetch("/api/Actions/" + id).then((res) => {
                res.text().then((text) => {
                    window.dispatchEvent(
                        new CustomEvent("ActionListing", {
                            detail: {
                                actions: JSON.parse(text),
                            },
                        }),
                    );
                });
            });
        }
    );
    var actionwindow = document.getElementById("actionList");
    actionwindow.classList.toggle("show");

    console.log(actionwindow.style.getPropertyValue("visibility"));
    //actionwindow.classList.add

}
