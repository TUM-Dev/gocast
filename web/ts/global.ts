
export async function postData(url = '', data = {}) {
    const response = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(data)
    });
    return response;
}

export function Get(yourUrl) {
    let HttpReq = new XMLHttpRequest();
    HttpReq.open("GET", yourUrl, false);
    HttpReq.send(null);
    return HttpReq.responseText;
}

export function showMessage(msg: string) {
    let alertBox: HTMLElement = document.getElementById("alertBox");
    let alertText: HTMLSpanElement = document.getElementById("alertText");
    alertText.innerText = msg;
    alertBox.classList.remove("hidden");
}

export function copyToClipboard(text: string) {
    const dummy = document.createElement("input");
    document.body.appendChild(dummy);
    dummy.value = text;
    dummy.select();
    document.execCommand("copy");
    document.body.removeChild(dummy);
}

export function hideCourse(id: number, name: string) {
    let hidden: Array<Array<string>> = localStorage.getItem("hiddenCourses") ? JSON.parse(localStorage.getItem("hiddenCourses")) : new Array<Array<string>>();
    if (!(hidden.indexOf([id.toString(), name]) !== -1)) {
        hidden.push([id.toString(), name]);
        localStorage.setItem("hiddenCourses", JSON.stringify(hidden));
    }
    document.location.reload();
}

export function unhideCourse(id: string, name: string) {
    let hidden: Array<Array<string>> = localStorage.getItem("hiddenCourses") ? JSON.parse(localStorage.getItem("hiddenCourses")) : new Array<Array<string>>();
    let newHidden: Array<Array<string>> = hidden.filter(e => {
        return (e[0] !== id);
    })
    localStorage.setItem("hiddenCourses", JSON.stringify(newHidden));
    document.location.reload();
}

export function toggleColorScheme() {
    //initial theme preference:
    let darkTheme: boolean = localStorage.getItem("darkTheme") ? JSON.parse(localStorage.getItem("darkTheme")) : true;
    //store opposite
    localStorage.setItem("darkTheme", JSON.stringify(!darkTheme));
    //set opposite class
    if (!darkTheme) {
        document.documentElement.classList.add("dark");
    } else {
        document.documentElement.classList.remove("dark");
    }
}

export function initHiddenCourses() {
    let el = document.getElementById("hiddenCoursesText");
    if (!el){
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
    }
    let hidden: Array<Array<string>> = localStorage.getItem("hiddenCourses") ? JSON.parse(localStorage.getItem("hiddenCourses")) : new Array<Array<string>>();
    const hiddenCoursesRestoreList = document.getElementById("hiddenCoursesRestoreList") as HTMLUListElement;
    const hiddenCoursesText = document.getElementById("hiddenCoursesText") as HTMLParagraphElement;
    hidden?.forEach(h => {
        const liElem = document.createElement("li");
        liElem.classList.add("hover:text-1", "cursor-pointer");
        liElem.innerText = "restore " + h[1];
        liElem.onclick = function () {
            unhideCourse(h[0], h[1]);
        }
        hiddenCoursesRestoreList.appendChild(liElem);
        const elems = document.getElementsByClassName("course" + h[0]);
        for (let i = 0; i < elems.length; i++) {
            elems[i].classList.add("hidden");
        }
    })
    if (hidden.length !== 0) {
        hiddenCoursesText.innerText = hidden.length + " hidden courses";
    }
}

window.onload = function () {
    initHiddenCourses();
}
