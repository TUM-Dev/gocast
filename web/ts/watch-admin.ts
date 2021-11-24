// @ts-nocheck
import {postData} from "./global";

export namespace WatchAdmin {
    export function usePreset(cID: number, lectureHallID: number, presetID: number, streamID: number) {
        (document.getElementById("presetImage" + lectureHallID + "-" + presetID) as HTMLImageElement).classList.add("animate-pulse")
        postData("/api/course/" + cID + "/switchPreset/" + lectureHallID + "/" + presetID + "/" + (document.getElementById("streamID") as HTMLInputElement).value).then(
            function () {
                (document.getElementById("presetImage" + lectureHallID + "-" + presetID) as HTMLImageElement).classList.remove("animate-pulse")
            }
        )
    }
}
