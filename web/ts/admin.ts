import { postData, showMessage } from "./global";
import { StatusCodes } from "http-status-codes";

class Admin {}

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
