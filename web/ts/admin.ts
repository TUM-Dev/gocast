import { Delete, postData, showMessage } from "./global";
import { StatusCodes } from "http-status-codes";

class Admin {}

export async function createLectureHall(
    name: string,
    streamProtocol: number,
    combIP: string,
    presIP: string,
    camIP: string,
    cameraIp: string,
    pwrCtrlIp: string,
) {
    return postData("/api/createLectureHall", {
        name,
        streamProtocol,
        presIP,
        camIP,
        combIP,
        cameraIp,
        pwrCtrlIp,
    }).then((e) => {
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

export async function createTestCourse() {
    await postData("/api/createTestCourse", {}).then((data) => {
        if (data.status === StatusCodes.OK) {
            showMessage("Test course was created successfully. Reload the page to see it.");
        } else {
            showMessage("There was an error creating the test course: " + data.body);
        }
    });
}
