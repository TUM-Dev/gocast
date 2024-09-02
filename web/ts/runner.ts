/** Make DELETE call to /api/runner/:hostname with given runner-hostname */
export async function deleteRunner(hostname: string) {
    return await fetch("/api/runners/" + hostname, {
        method: "DELETE",
    });
}

export function getFailedAction() {
    window.dispatchEvent(new CustomEvent("load-failures"));
    fetch("/api/Actions/failed").then(
        (res) => {
            res.text().then((text) => {
                console.log(text);
            });
        },
    );
}