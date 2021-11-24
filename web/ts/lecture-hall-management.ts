import { postData } from './global'

export function takeSnapshot(lectureHallID: number, presetID: number) {
    if (confirm("Do you want to take a snapshot? Make sure no lecture is live in this lecture hall.")) {
        (document.getElementById("presetImage" + lectureHallID + "-" + presetID) as HTMLImageElement).classList.add("animate-pulse")
        postData("/api/takeSnapshot/" + lectureHallID + "/" + presetID).then(
            function (res) {
                (document.getElementById("presetImage" + lectureHallID + "-" + presetID) as HTMLImageElement).classList.remove("animate-pulse")
                if (res.ok) {
                    res.text().then(function (responseString) {
                        (document.getElementById("presetImage" + lectureHallID + "-" + presetID) as HTMLImageElement).src = JSON.parse(responseString)["path"]
                    })
                }
            }
        )
    }
}
