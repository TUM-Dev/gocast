import { Notifications } from "../notifications";
import { ToggleableElement } from "../utilities/ToggleableElement";

export function header() {
    return {
        userContext: new ToggleableElement([["themePicker", new ToggleableElement()]]),

        notifications: new Notifications(),
        notification: new ToggleableElement(),
        toggleNotification(set?: boolean) {
            this.notification.toggle(set);
            this.notifications.writeToStorage(true);
        },
    };
}
