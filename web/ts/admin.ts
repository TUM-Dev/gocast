import { Delete, postData, showMessage } from "./global";
import { StatusCodes } from "http-status-codes";

class Admin {}

export class AdminUserList {
    readonly rowsPerPage: number;

    numberOfPages: number;
    currentIndex: number;

    list: object[]; // Pre-loaded users
    currentPage: object[]; // Subset of list

    showSearchResults: boolean;
    searchLoading: boolean;
    searchInput: string;
    roles: number;

    constructor(usersAsJson: object[]) {
        this.list = usersAsJson;
        this.rowsPerPage = 10;
        this.showSearchResults = false;
        this.currentIndex = 0;
        this.searchInput = "";
        this.roles = -1;
        this.numberOfPages = Math.ceil(this.list.length / this.rowsPerPage);
        this.updateVisibleRows();
    }

    async search() {
        if (this.searchInput.length < 3 && this.roles == -1) {
            this.showSearchResults = false;
            this.updateVisibleRows();
            return;
        } else {
            this.searchLoading = true;
            fetch("/api/searchUser?q=" + this.searchInput + "&r=" + this.roles)
                .then((response) => {
                    this.searchLoading = false;
                    if (!response.ok) {
                        throw new Error(response.statusText);
                    }
                    return response.json();
                })
                .then((r) => {
                    if (this.roles != -1) {
                        this.currentPage = r.filter((obj) => {
                            return obj.role == this.roles;
                        }); // show all results on page one.
                    } else {
                        this.currentPage = r;
                    }
                    this.showSearchResults = true;
                })
                .catch((err) => {
                    console.error(err);
                    this.showSearchResults = false;
                    this.updateVisibleRows();
                });
        }
    }

    clearSearch() {
        this.showSearchResults = false;
        this.searchLoading = false;
        this.updateVisibleRows();
        this.searchInput = "";
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
        this.currentPage = this.list.slice(
            this.currentIndex * this.rowsPerPage,
            this.currentIndex * this.rowsPerPage + this.rowsPerPage,
        );
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

export async function updateUser(userID: number, role: number) {
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

export async function updateText(id: number, name: string, content: string) {
    await fetch("/api/texts/" + id, {
        method: "PUT",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify({
            name: name,
            content: content,
            type: 1, // model.TEXT_MARKDOWN
        }),
    })
        .then((res) => {
            if (res.status !== StatusCodes.OK) {
                throw new Error(res.statusText);
            }
        })
        .catch((err) => {
            showMessage("There was an error updating the text: " + err);
        })
        .then(() => {
            showMessage(`Successfully updated "${name}"`);
        });
}

export async function requestSubtitles(streamID: number, language: string) {
    await postData(`/api/stream/${streamID}/subtitles`, { language })
        .then((res) => {
            if (!res.ok) {
                throw Error(res.statusText);
            }
            return;
        })
        .catch((err) => {
            console.error(err);
        });
}

export function impersonate(userID: number): Promise<boolean> {
    return fetch("/api/users/impersonate", {
        method: "POST",
        body: JSON.stringify({ id: userID }),
        headers: {
            "Content-Type": "application/json",
        },
    }).then((r) => {
        return r.status === StatusCodes.OK;
    });
}
