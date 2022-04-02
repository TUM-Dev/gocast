export class Notifications {
    notifications: Notification[] = [];

    constructor() {
        this.notifications = [];
    }

    hasNewNotifications(): boolean {
        return this.notifications.some(notification => !notification.read);
    }

    fetchNotifications(): void {
        let lastNotificationFetch: Date = new Date(parseInt(localStorage.getItem('lastNotificationFetch') || '0'));
        // fetch every 10 minutes at most:
        console.log('lastNotificationFetch: ' + lastNotificationFetch);
        console.log(new Date().getTime() - lastNotificationFetch.getTime());
        if (new Date().getTime() - lastNotificationFetch.getTime() > 1000 * 60 * 10) {
            this.notifications = [];
            fetch(`/api/notifications/`)
                .then(response => response.json() as Promise<Notification[]>)
                .then(data => {
                    console.log(data);
                    localStorage.setItem('lastNotificationFetch', new Date().getTime().toString());
                    localStorage.setItem('notifications', JSON.stringify(data));
                    return data;
                });
        } else {
            this.notifications = JSON.parse(localStorage.getItem('notifications') || '[]');
        }
    }
}

class Notification {
    ID: number;
    CreatedAt: Date;
    title: string | undefined;
    body: string;
    read: boolean;
}
