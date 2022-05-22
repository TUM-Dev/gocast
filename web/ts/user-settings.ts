const settingsAPIBaseURL = "/api/users/settings";

export enum UserSetting {
    Name = "name",
    Greeting = "greeting",
    EnableCast = "enableCast",
}

export function updatePreference(t: UserSetting, value: string|boolean): Promise<string> {
    return fetch(`${settingsAPIBaseURL}/${t}`, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify({ value }),
    }).then((response) => {
        if (response.status === 200) {
            return ""; // indicates success
        } else {
            return response.json();
        }
    });
}
