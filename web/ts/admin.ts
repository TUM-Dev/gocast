import {postData, showMessage} from './global'
import {StatusCodes} from "http-status-codes";

class Admin {

}

export default function printMoin(): number {
   return 42;
}

export function createLectureHall() {
    postData("/api/createLectureHall", {
        "name": (document.getElementById("newLectureHallName") as HTMLInputElement).value,
        "combIP": (document.getElementById("newLectureHallCombIP") as HTMLInputElement).value,
        "presIP": (document.getElementById("newLectureHallPresIP") as HTMLInputElement).value,
        "camIP": (document.getElementById("newLectureHallCamIP") as HTMLInputElement).value,
    }).then(e => {
        if (e.status === StatusCodes.OK) {
            window.location.reload()
        }
    })
}

export function createUser() {
    const userName: string = (document.getElementById("name") as HTMLInputElement).value
    const email: string = (document.getElementById("email") as HTMLInputElement).value
    postData("/api/createUser", {"name": userName, "email": email, "password": null})
        .then(data => {
            if (data.status === StatusCodes.OK) {
                showMessage("User was created successfully. Reload to see them.")
            } else {
                showMessage("There was an error creating the user: " + data.body)
            }
        })
}

export function deleteUser(id: number) {
    if (confirm("Confirm deleting user.")) {
        postData("api/deleteUser", {"id": id})
            .then(data => {
                    if (data.status === StatusCodes.OK) {
                        showMessage("User was deleted successfully.")
                        const row = document.getElementById("user" + id)
                        row.parentElement.removeChild(row)
                    } else {
                        showMessage("There was an error deleting the user: " + data.body)
                    }
                }
            )
    }
}
