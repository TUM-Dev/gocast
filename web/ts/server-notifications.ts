class ServerNotifications {
    constructor() {
        ["from", "expires"].forEach((value) => {
            Array.prototype.forEach.call(document.getElementsByClassName(value), function (el) {
                console.log(el.id);
                // @ts-ignore
                flatpickr(`#${el.id}`, { enableTime: true, time_24hr: true });
            });
        });
    }
}

window.onload = function () {
    new ServerNotifications();
};
