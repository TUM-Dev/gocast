import { Delete, patchData, postData, showMessage } from "./global";
import { StatusCodes } from "http-status-codes";

class Admin {}

export class User {
    readonly id: number;
    readonly name: string;
    readonly email: string;
    readonly role: number;
    readonly lrz_id: string;

    constructor(id: number, name: string, email: string, role: number, lrz_id: string) {
        this.id = id;
        this.name = name;
        this.email = email;
        this.role = role;
        this.lrz_id = lrz_id;
    }
}

export class AdminUserList {
    readonly rowsPerPage: number;

    numberOfPages: number;
    currentIndex: number;

    list: object[]; // Pre-loaded users
    currentPage: object[]; // Subset of list

    showSearchResults: boolean;
    searchLoading: boolean;
    searchInput: string;

    constructor(usersAsJson: object[]) {
        this.list = usersAsJson;
        this.rowsPerPage = 10;
        this.showSearchResults = false;
        this.currentIndex = 0;
        this.numberOfPages = Math.ceil(this.list.length / this.rowsPerPage);
        this.updateVisibleRows();
    }

    async search() {
        if (this.searchInput.length < 3) {
            this.showSearchResults = false;
            this.updateVisibleRows();
            return;
        }
        if (this.searchInput.length > 2) {
            this.searchLoading = true;
            fetch("/api/searchUser?q=" + this.searchInput)
                .then((response) => {
                    this.searchLoading = false;
                    if (!response.ok) {
                        throw new Error(response.statusText);
                    }
                    return response.json();
                })
                .then((r) => {
                    this.currentPage = r; // show all results on page one.
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

export class OrganizationsList {
    readonly rowsPerPage: number;

    numberOfPages: number;
    currentIndex: number;

    list: object[]; // Pre-loaded organizations
    currentPage: object[]; // Subset of list

    showSearchResults: boolean;
    searchLoading: boolean;
    searchInput: string;

    constructor(organizationsAsJson: object[]) {
        this.list = organizationsAsJson;
        this.rowsPerPage = 10;
        this.showSearchResults = false;
        this.currentIndex = 0;
        this.numberOfPages = Math.ceil(this.list.length / this.rowsPerPage);
        this.updateVisibleRows();
    }

    async search() {
        if (this.searchInput.length < 3) {
            this.showSearchResults = false;
            this.updateVisibleRows();
            return;
        }
        if (this.searchInput.length > 2) {
            this.searchLoading = true;
            fetch("/api/organizations?q=" + this.searchInput)
                .then((response) => {
                    this.searchLoading = false;
                    if (!response.ok) {
                        throw new Error(response.statusText);
                    }
                    return response.json();
                })
                .then((r) => {
                    this.currentPage = r; // show all results on page one.
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

export async function createOrganization(name: string, org_type: string, admin_email: string, parent_id: number = null) {
    console.log("Creating organization", name, org_type, admin_email, parent_id);
    const response = await postData("/api/organizations", {
        name: name,
        org_type: org_type,
        admin_email: admin_email,
        parent_id: parent_id,
    });

    if (response.status === StatusCodes.OK) {
        showMessage("Organization was created successfully. Reload to see changes.");
    } else {
        const errorData = await response.json();
        showMessage(`Error creating the organization: ${errorData.error}`);
    }
}

export async function createTokenForOrganization(organizationID: number) {
    const response = await postData(`/api/organizations/${organizationID}/token`);

    if (response.status === StatusCodes.OK) {
        const data = await response.json();
        const token = data.token;
        if (navigator.clipboard) {
            await navigator.clipboard.writeText(token);
            showMessage(`Token was created successfully and copied to your clipboard.`);
        } else {
            showMessage(
                `Token was created successfully but could not be copied to clipboard because the Clipboard API is not available in this context. Copy the token manually: ${token}`,
            );
        }
    } else {
        const errorData = await response.json();
        showMessage(`Error creating the token: ${errorData.error}`);
    }
}

export async function updateOrganization(organizationID: number, name: string) {
    return patchData(`/api/organizations/${organizationID}`, { name }).then((e) => {
        if (e.status === StatusCodes.OK) {
            showMessage("Organization was updated successfully. Reload to see changes.");
        } else {
            showMessage("There was an error updating the organization: " + e.body);
        }
    });
}

export async function toggleSharedStatus(workerID) {
    return postData(`/api/workers/${workerID}/toggleShared`, { workerID }).then((e) => {
        if (e.status === StatusCodes.OK) {
            showMessage("Worker was updated successfully.");
        } else {
            showMessage("There was an error updating the worker.");
        }
    });
}

export async function deleteOrganization(organizationID: number) {
    if (
        confirm(
            "Do you really want to remove this organization? This will also remove all associated lecture halls and users!",
        )
    ) {
        const response = await fetch(`/api/organizations/${organizationID}`, { method: "DELETE" });

        if (response.ok) {
            showMessage("Organization removed successfully. Reload to see changes.");
        } else {
            const errorData = await response.json();
            showMessage(`Error deleting the organization: ${errorData.error}`);
        }
    }
}

export async function addOrganizationAdmin(organizationID: number, email: string, id: number, lrz_id: string) {
    if (
        confirm(
            "Do you really want to add this user as an maintainer? If you do, they will be granted full admin priviledges for all resources of organization!",
        )
    ) {
        const adminDetail = email ? { email } : id ? { id } : { lrz_id };

        const response = await fetch(`/api/organizations/${organizationID}/admins`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify(adminDetail),
        });

        if (response.ok) {
            showMessage("Maintainer added successfully. Reload to see changes.");
        } else {
            const errorData = await response.json();
            showMessage(`Error adding the admin: ${errorData.error}`);
        }
    }
}

export async function deleteOrganizationAdmin(organizationID: number, adminID: number) {
    if (
        confirm(
            "Do you really want to remove this maintainer? If you do, they will lose admin priviledges for the organization's resources! (If you are the only admin, you will lose access to the organization as well!)",
        )
    ) {
        const response = await fetch(`/api/organizations/${organizationID}/admins/${adminID}`, { method: "DELETE" });

        if (response.ok) {
            showMessage("Maintainer removed successfully. Reload to see changes.");
        } else {
            const errorData = await response.json();
            showMessage(`Error removing the admin: ${errorData.error}`);
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
