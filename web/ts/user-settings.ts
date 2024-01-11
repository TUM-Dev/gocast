import { StatusCodes } from "http-status-codes";
import Alpine from "alpinejs";

const settingsAPIBaseURL = "/api/users/settings";

export enum UserSetting {
    Name = "name",
    Greeting = "greeting",
    PlaybackSpeeds = "playbackSpeeds",
    SeekingTime = "seekingTime",
    CustomSpeeds = "customSpeeds",
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

export function sanitizeInputSpeed(value: number): number {
    if (value > 5) {
        return 5;
    } else if (value <= 0) {
        return 0.01;
    }
    return Math.round(value * 100) / 100;
}

export function checkInputSpeed(value: number, currentSpeeds: number[]) {
    const defaultSpeeds = [0.25, 0.5, 0.75, 1, 1.5, 1.5, 1.75, 2, 2.5, 3, 3.5];
    return !defaultSpeeds.includes(value) && !currentSpeeds.includes(value) && currentSpeeds.length < 3;
}
