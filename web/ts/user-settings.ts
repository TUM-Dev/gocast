import { StatusCodes } from "http-status-codes";

const settingsAPIBaseURL = "/api/users/settings";

export enum UserSetting {
    Name = "name",
    Greeting = "greeting",
    PlaybackSpeeds = "playbackSpeeds",
}

export function updatePreference(t: UserSetting, value: string | boolean | number[]): Promise<string> {
    return fetch(`${settingsAPIBaseURL}/${t}`, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify({ value }),
    }).then((response) => {
        if (response.status === StatusCodes.OK) {
            return ""; // indicates success
        } else {
            return response.json();
        }
    });
}
