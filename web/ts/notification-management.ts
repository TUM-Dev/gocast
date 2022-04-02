import {Notification} from "./notifications"

export function createNotification(body: string, target: number, title: string | undefined = undefined): void {
    const notification = new Notification(title, body, target);
    fetch("/api/notifications/", {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify(notification)
    }).then(r => r.json()).then(r => {
        window.location.reload();
    });
}

export function deleteNotification(id: number): void {
    console.log("Deleting notification with id: " + id);
    fetch("/api/notifications/" + id, {
        method: "DELETE"
    }).then(r => r.json()).then(r => {
        window.location.reload();
    });
}
