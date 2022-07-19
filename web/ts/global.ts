import { StatusCodes } from "http-status-codes";

export * from "./notifications";
export * from "./user-settings";
export * from "./start-page";

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

export function hideCourse(id: number, name: string) {
    const hidden: Array<Array<string>> = localStorage.getItem("hiddenCourses")
        ? JSON.parse(localStorage.getItem("hiddenCourses"))
        : new Array<Array<string>>();
    if (!(hidden.indexOf([id.toString(), name]) !== -1)) {
        hidden.push([id.toString(), name]);
        localStorage.setItem("hiddenCourses", JSON.stringify(hidden));
    }
    document.location.reload();
}

export function unhideCourse(id: string) {
    const hidden: Array<Array<string>> = localStorage.getItem("hiddenCourses")
        ? JSON.parse(localStorage.getItem("hiddenCourses"))
        : new Array<Array<string>>();
    const newHidden: Array<Array<string>> = hidden.filter((e) => {
        return e[0] !== id;
    });
    localStorage.setItem("hiddenCourses", JSON.stringify(newHidden));
    document.location.reload();
}

export function pinCourse(id: number) {
    postData(`/api/users/pinCourse`, { courseID: id }).then((response: Response) => {
        if (response.status !== StatusCodes.OK) {
            showMessage("There was an error pinning the course: " + response.body);
        }
        document.location.reload();
    });
}

export function unpinCourse(id: number) {
    postData(`/api/users/unpinCourse`, { courseID: id }).then((response: Response) => {
        if (response.status !== StatusCodes.OK) {
            showMessage("There was an error unpinning the course: " + response.body);
        }
        document.location.reload();
    });
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

export function initHiddenCourses() {
    const el = document.getElementById("hiddenCoursesText");
    if (!el) {
        return;
    }
    el.onclick = function () {
        const clickableParent: HTMLElement = document.getElementById("hiddenCoursesRestoreList")?.parentElement;
        if (clickableParent === undefined || clickableParent === null) {
            return; // not on index page
        }
        if (clickableParent.classList.contains("hidden")) {
            clickableParent.classList.remove("hidden");
        } else {
            clickableParent.classList.add("hidden");
        }
    };
    const hidden: Array<Array<string>> = localStorage.getItem("hiddenCourses")
        ? JSON.parse(localStorage.getItem("hiddenCourses"))
        : new Array<Array<string>>();
    const hiddenCoursesRestoreList = document.getElementById("hiddenCoursesRestoreList") as HTMLUListElement;
    const hiddenCoursesText = document.getElementById("hiddenCoursesText") as HTMLParagraphElement;
    hidden?.forEach((h) => {
        const liElem = document.createElement("li");
        liElem.classList.add("hover:text-1", "cursor-pointer");
        liElem.innerText = "restore " + h[1];
        liElem.onclick = function () {
            unhideCourse(h[0]);
        };
        hiddenCoursesRestoreList.appendChild(liElem);
        const elems = document.getElementsByClassName("course" + h[0]);
        for (let i = 0; i < elems.length; i++) {
            elems[i].classList.add("hidden");
        }
    });
    if (hidden.length !== 0) {
        hiddenCoursesText.innerText = hidden.length + " hidden courses";
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

window.onload = function () {
    initHiddenCourses();
};
