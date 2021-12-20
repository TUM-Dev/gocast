class Admin {}

function createLectureHall(name: string, combIP: string, presIP: string, camIP: string) {
    postData("/api/createLectureHall", { name, presIP, camIP, combIP }).then((e) => {
        if (e.status === 200) {
            window.location.reload();
        }
    });
}

function createUser() {
    const userName: string = (document.getElementById("name") as HTMLInputElement).value;
    const email: string = (document.getElementById("email") as HTMLInputElement).value;
    postData("/api/createUser", { name: userName, email: email, password: null }).then((data) => {
        if (data.status === 200) {
            showMessage("User was created successfully. Reload to see them.");
        } else {
            showMessage("There was an error creating the user: " + data.body);
        }
    });
}

function deleteUser(deletedUserID: number) {
    if (confirm("Confirm deleting user.")) {
        postData("api/deleteUser", { id: deletedUserID }).then((data) => {
            if (data.status === 200) {
                showMessage("User was deleted successfully.");
                const row = document.getElementById("user" + deletedUserID);
                row.parentElement.removeChild(row);
            } else {
                showMessage("There was an error deleting the user: " + data.body);
            }
        });
    }
}
