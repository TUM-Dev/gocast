function usePreset(cID: number, lectureHallID: number, presetID: number) {
    const streamID = (document.getElementById("streamID") as HTMLInputElement).value;
    const presetPath = "/api/course/" + cID + "/switchPreset/" + lectureHallID + "/" + presetID + "/" + streamID;
    const presetClassList = (document.getElementById("presetImage" + lectureHallID + "-" + presetID) as HTMLImageElement).classList;

    presetClassList.add("animate-pulse");

    postData(presetPath).then(
        () => {
            presetClassList.remove("animate-pulse");
        }
    )
}
