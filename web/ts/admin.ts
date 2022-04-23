import { Delete, postData, showMessage } from "./global";
import { StatusCodes } from "http-status-codes";

class Admin {}

export class AdminUserList {
    readonly rowsPerPage: number;
    readonly numberOfPages: number;

    currentIndex: number;

    constructor() {
        this.rowsPerPage = 10;
        this.currentIndex = 0;
        this.updateVisibleRows();

        this.numberOfPages = Math.ceil(document.getElementById("admin-user-list").children.length / this.rowsPerPage);
    }

    currentIndexString(): string {
        return `${this.currentIndex + 1}/${this.numberOfPages}`;
    }

    prevDisabled(): boolean {
        return this.currentIndex === 0;
    }

    nextDisabled(): boolean {
        return this.currentIndex === this.numberOfPages - 1;
    }

    next() {
        this.currentIndex = (this.currentIndex + 1) % this.numberOfPages;
        this.updateVisibleRows();
    }

    prev() {
        this.currentIndex = (this.currentIndex - 1) % this.numberOfPages;
        this.updateVisibleRows();
    }

    updateVisibleRows() {
        const table = document.getElementById("admin-user-list");
        const minIndex = this.currentIndex * this.rowsPerPage;
        const maxIndex = this.currentIndex * this.rowsPerPage + this.rowsPerPage - 1;
        Array.from(table.children).forEach((row: HTMLElement) => {
            const idx = parseInt(row.dataset.userlistIndex);
            if (idx < minIndex || idx > maxIndex) {
                row.classList.add("hidden");
            } else {
                row.classList.remove("hidden");
            }
        });
    }
}

export async function createLectureHall(
    name: string,
    combIP: string,
    presIP: string,
    camIP: string,
    cameraIp: string,
    pwrCtrlIp: string,
) {
    return postData("/api/createLectureHall", { name, presIP, camIP, combIP, cameraIp, pwrCtrlIp }).then((e) => {
        return e.status === StatusCodes.OK;
    });
}

export async function deleteLectureHall(lectureHallID: number) {
    if (confirm("Do you really want to remove this lecture hall?")) {
        try {
            await Delete("/api/lectureHall/" + lectureHallID);
            document.location.reload();
        } catch (e) {
            alert("Something went wrong while deleting!");
        }
    }
}
export function createUser() {
    const userName: string = (document.getElementById("name") as HTMLInputElement).value;
    const email: string = (document.getElementById("email") as HTMLInputElement).value;
    postData("/api/createUser", { name: userName, email: email, password: null }).then((data) => {
        if (data.status === StatusCodes.OK) {
            showMessage("User was created successfully. Reload to see them.");
        } else {
            showMessage("There was an error creating the user: " + data.body);
        }
    });
}

export function deleteUser(deletedUserID: number) {
    if (confirm("Confirm deleting user.")) {
        postData("/api/deleteUser", { id: deletedUserID }).then((data) => {
            if (data.status === StatusCodes.OK) {
                showMessage("User was deleted successfully.");
                const row = document.getElementById("user" + deletedUserID);
                row.parentElement.removeChild(row);
            } else {
                showMessage("There was an error deleting the user: " + data.body);
            }
        });
    }
}

async function updateUser(userID: number, role: number) {
    let success = true;
    await fetch("/api/users/update", {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify({
            id: userID,
            role: role,
        }),
    })
        .then((res) => {
            if (res.status !== StatusCodes.OK) {
                success = false;
                showMessage("There was an error updating the user: " + res.body);
            }
        })
        .catch((err) => {
            success = false;
            showMessage("There was an error updating the user: " + err);
        });
    return success ? role : -1;
}
