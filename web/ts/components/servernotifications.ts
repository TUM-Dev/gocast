import { NotificationAPI } from "../api/notifications";

export function serverNotifications() {
    return {
        serverNotifications: [],
        init() {
            this.load();
        },

        async load() {
            this.serverNotifications = await NotificationAPI.getServerNotifications();
        },
    };
}
