import { get } from "../utilities/fetch-wrappers";

/**
 * REST API Wrapper for /api/notifications
 */
export const NotificationAPI = {
    async getServerNotifications() {
        return get("/api/notifications/server");
    },
};
