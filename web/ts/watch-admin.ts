function usePreset(lectureHallID: number, presetID: number) {
    (document.getElementById("presetImage" + lectureHallID + "-" + presetID) as HTMLImageElement).classList.add("animate-pulse")
    postData("/api/switchPreset/" + lectureHallID + "/" + presetID + "/" + (document.getElementById("streamID") as HTMLInputElement).value).then(
        function (res) {
            (document.getElementById("presetImage" + lectureHallID + "-" + presetID) as HTMLImageElement).classList.remove("animate-pulse")
        }
    )
}