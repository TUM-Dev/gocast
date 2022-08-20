import { StatusCodes } from "http-status-codes";

export * from "./notifications";
export * from "./user-settings";
export * from "./start-page";

import { DateTime, Duration } from "luxon";
export { DateTime };

export async function putData(url = "", data = {}) {
    return await fetch(url, {
        method: "PUT",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify(data),
    });
}

export async function postData(url = "", data = {}) {
    return await fetch(url, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify(data),
    });
}

export async function Delete(url = "") {
    return await fetch(url, {
        method: "DELETE",
    });
}

export function Get(yourUrl) {
    const HttpReq = new XMLHttpRequest();
    HttpReq.open("GET", yourUrl, false);
    HttpReq.send(null);
    return HttpReq.responseText;
}

export function showMessage(msg: string) {
    const alertBox: HTMLElement = document.getElementById("alertBox");
    const alertText: HTMLSpanElement = document.getElementById("alertText");
    alertText.innerText = msg;
    alertBox.classList.remove("hidden");
}

/**
 * Copies a string to the clipboard using clipboard API.
 * @param text the string that is copied to the clipboard.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
    return navigator.clipboard.writeText(text).then(
        () => {
            return true;
        },
        () => {
            return false;
        },
    );
}

export enum CourseFlag {
    Pinned = 1,
    Hidden
}

// set/unsets the pinned/hidden flag for a signed-in user
export async function updateCourseFlag(courseID: number, flag: CourseFlag, value: boolean) {
    const r = await postData("/api/users/updateCourseFlag", { courseID, flag, value });
    if (!r.ok)
        showMessage(await r.text());
    document.location.reload();
}

/**
 * Mirrors a tree (reverses the order of its "leaves") in the DOM.
 */
export function mirror(parent: Element, levelSelectors: string[], levelIndex = 0) {
    const children = parent.querySelectorAll(levelSelectors[levelIndex]); // querySelectorAll returns static node list

    // if this is not a leaf, recurse
    if (levelIndex + 1 < levelSelectors.length)
        children.forEach((child) => mirror(child, levelSelectors, levelIndex + 1));

    // mirror the direct children
    const placeholder = document.createElement("div");
    for (let childI = 0; childI * 2 + 1 < children.length; childI++) {
        const a = children[childI];
        const b = children[children.length - childI - 1];
        a.replaceWith(placeholder);
        b.replaceWith(a);
        placeholder.replaceWith(b);
    }
}

// Adapted from https://codepen.io/harsh/pen/KKdEVPV
export function timer(expiry: string, leadingZero: boolean) {
    const date = new Date(expiry);
    return {
        expiry: date,
        remaining: null,
        init() {
            this.setRemaining();
            setInterval(() => {
                this.setRemaining();
            }, 1000);
        },
        setRemaining() {
            const diff = this.expiry - new Date().getTime();
            if (diff >= 0) {
                this.remaining = parseInt(String(diff / 1000));
            } else {
                this.remaining = 0;
            }
        },
        days() {
            return {
                value: this.remaining / 86400,
                remaining: this.remaining % 86400,
            };
        },
        hours() {
            return {
                value: this.days().remaining / 3600,
                remaining: this.days().remaining % 3600,
            };
        },
        minutes() {
            return {
                value: this.hours().remaining / 60,
                remaining: this.hours().remaining % 60,
            };
        },
        seconds() {
            return {
                value: this.minutes().remaining,
            };
        },
        format(value) {
            if (leadingZero) {
                return ("0" + parseInt(value)).slice(-2);
            } else {
                return parseInt(value);
            }
        },
        time() {
            return {
                days: this.format(this.days().value),
                hours: this.format(this.hours().value),
                minutes: this.format(this.minutes().value),
                seconds: this.format(this.seconds().value),
            };
        },
    };
}

function nextRelativeTimeUpdate(base: DateTime, time: DateTime, options: Object,
                                delta: Duration = Duration.fromMillis(1), d0: Duration = Duration.fromMillis(1000)) {
    // avoid constructing a whole new o every time
    let o = { ...options, base };
    function strForBase(b) {
        o.base = b;
        return time.toRelative(o);
    }

    const str0 = strForBase(base);
    let dMax = d0;
    while (str0 === strForBase(base.plus(dMax))) {
        dMax = dMax.plus(dMax); // double
    }

    let dMin = Duration.fromMillis(0);
    while (dMax - dMin > delta) {
        const d = Duration.fromMillis((dMax + dMin) / 2);
        if (strForBase(base.plus(d)) == str0) dMin = d;
        else dMax = d;
    }

    return dMax;
}

export const dynamicRelativeTime = (time: DateTime, options: Object) => ({
    init() {
        const helper = () => {
            const now = DateTime.now();
            this.isFuture = time >= now;
            this.relativeTime = time.toRelative({ ...options, base: now });
            const wait = nextRelativeTimeUpdate(now, time, options);
            setTimeout(() => { helper(); }, wait.toMillis());
        }
        helper();
    },
    relativeTime: "",
    isFuture: false,
});

// getLoginReferrer returns "/" if document.referrer === "http[s]://<hostname>:<port>/login" and document.referrer if not
export function getLoginReferrer(): string {
    const lastLocation = document.referrer.split("/"),
        protocol = lastLocation[0],
        host = lastLocation[2];

    if (
        window.location.protocol !== protocol ||
        window.location.host !== host ||
        document.referrer === window.location.origin + "/login"
    ) {
        return window.location.origin + "/";
    }

    return document.referrer;
}

// TypeScript Mapping of model.VideoSection
export type Section = {
    ID?: number;
    description: string;

    startHours: number;
    startMinutes: number;
    startSeconds: number;

    streamID: number;
    friendlyTimestamp?: string;
    fileID?: number;
};
