class Admin {}

async function createLectureHall(
    name: string,
    combIP: string,
    presIP: string,
    camIP: string,
    cameraIp: string,
    pwrCtrlIp: string,
) {
    return postData("/api/createLectureHall", { name, presIP, camIP, combIP, cameraIp, pwrCtrlIp }).then((e) => {
        if (e.status === 200) {
            return true;
        }
        return false;
    });
}

async function deleteLectureHall(lectureHallID: number) {
    if (confirm("Do you really want to remove this lecture hall?")) {
        try {
            await Delete("/api/lectureHall/" + lectureHallID);
            document.location.reload();
        } catch (e) {
            alert("Something went wrong while deleting!");
        }
    }
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
            if (res.status !== 200) {
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
