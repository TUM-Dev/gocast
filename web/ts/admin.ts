import { Delete, postData, showMessage } from "./global";
import { StatusCodes } from "http-status-codes";

class Admin {}

export type LectureHallResult = { ok: boolean; error?: string };

export async function createLectureHall(
    name: string,
    streamProtocol: number,
    combIp: string,
    presIp: string,
    camIp: string,
    cameraIp: string,
    pwrCtrlIp: string,
): Promise<LectureHallResult> {
    const res = await postData("/api/createLectureHall", {
        name,
        streamProtocol,
        presIp,
        camIp,
        combIp,
        cameraIp,
        pwrCtrlIp,
    });
    return toLectureHallResult(res);
}

export async function updateLectureHall(
    id: number,
    name: string,
    streamProtocol: number,
    combIp: string,
    presIp: string,
    camIp: string,
    cameraIp: string,
    pwrCtrlIp: string,
): Promise<LectureHallResult> {
    const res = await fetch(`/api/lectureHall/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, streamProtocol, presIp, camIp, combIp, cameraIp, pwrCtrlIp }),
    });
    return toLectureHallResult(res);
}

/**
 * Turns a lecture hall API response into a result carrying the server's own message,
 * so the form can say why a save was rejected instead of just that it was.
 */
async function toLectureHallResult(res: Response): Promise<LectureHallResult> {
    if (res.status === StatusCodes.OK) {
        return { ok: true };
    }
    let error = `Request failed with status ${res.status}.`;
    try {
        const body = await res.json();
        if (typeof body === "string") {
            error = body;
        } else if (body?.message) {
            error = body.error ? `${body.message}: ${body.error}` : body.message;
        }
    } catch {
        // No JSON body - keep the status based message.
    }
    return { ok: false, error };
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
