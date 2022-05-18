import { postData } from "./global";
import { StatusCodes } from "http-status-codes";

export function usePreset(cID: number, lectureHallID: number, presetID: number) {
    const streamID = (document.getElementById("streamID") as HTMLInputElement).value;
    const presetPath = "/api/course/" + cID + "/switchPreset/" + lectureHallID + "/" + presetID + "/" + streamID;
    const presetClassList = (
        document.getElementById("presetImage" + lectureHallID + "-" + presetID) as HTMLImageElement
    ).classList;

    presetClassList.add("animate-pulse");

    postData(presetPath).then(() => {
        presetClassList.remove("animate-pulse");
    });
}

class Issue {
    readonly name: string;
    readonly phone: string;
    readonly email: string;
    readonly description: string;
    readonly categories: string[];

    constructor(name: string, phone: string, email: string, description: string, categories: string[]) {
        this.name = name;
        this.phone = phone;
        this.email = email;
        this.description = description;
        this.categories = categories;
    }
}

export function sendIssue(
    streamID: string,
    categories: string[],
    name: string,
    phone: string,
    email: string,
    description: string,
): Promise<boolean> {
    const issue = new Issue(name, phone, email, description, categories);
    return postData(`/api/stream/${streamID}/issue`, issue).then((response) => {
        return response.status === StatusCodes.OK;
    });
}
