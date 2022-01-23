import { postData } from "./global";
import { StatusCodes } from "http-status-codes";
import videojs from "video.js";
import noUiSlider from "nouislider";

let slider;
let player;

export const initLecturePlayer = function () {
    player = videojs("my-video", {
        html5: {
            hls: {
                overrideNative: true,
            },
            nativeVideoTracks: false,
            nativeAudioTracks: false,
            nativeTextTracks: false,
        },
    });
    player.play();
    player.one("loadedmetadata", function () {
        slider = document.getElementById("sliderNew");
        noUiSlider.create(slider, {
            format: {
                to: function (value) {
                    return sToTime(value);
                },
                from: function (value): number {
                    return Number(value);
                },
            },
            start: [0, player.duration()],
            connect: true,
            range: {
                min: 0,
                max: player.duration(),
            },
        });

        const tooltipInputs = [makeTT(0, slider), makeTT(1, slider)];
        slider.noUiSlider.on("update", function (values, handle) {
            tooltipInputs[handle].value = values[handle];
            player.currentTime(timeToS(values[handle]));
        });
    });
};

export function submitNewUnit(lectureID: number) {
    //convert from and to milliseconds relatively to beginning of video.
    const from = timeToS(slider.noUiSlider.get()[0]) * 1000;
    const to = timeToS(slider.noUiSlider.get()[1]) * 1000;
    const title = (document.getElementById("newUnitTitle") as HTMLInputElement).value;
    const description = (document.getElementById("newUnitDescription") as HTMLInputElement).value;
    postData("/api/addUnit", {
        lectureID: lectureID,
        from: from,
        to: to,
        title: title,
        description: description,
    }).then((data) => {
        if (data.status == StatusCodes.OK) {
            window.location.reload();
        } else {
            data.text().then((text) => {
                alert("error! status: " + data.status + ", message: " + text);
            });
        }
    });
    return false;
}

export function submitCut(lectureID: number, courseID: number) {
    const from = timeToS(slider.noUiSlider.get()[0]) * 1000;
    const to = timeToS(slider.noUiSlider.get()[1]) * 1000;
    postData("/api/submitCut", {
        lectureID: lectureID,
        from: from,
        to: to,
    }).then((data) => {
        if (data.status == StatusCodes.OK) {
            window.location.replace("/admin/course/" + courseID);
        } else {
            data.text().then((text) => {
                alert("error! status: " + data.status + ", message: " + text);
            });
        }
    });
    return false;
}

export function deleteUnit(unitID: number) {
    postData("/api/deleteUnit/" + unitID).then((r) => {
        if (r.status == StatusCodes.OK) {
            window.location.reload();
        }
    });
}

export function timeToS(s) {
    const parts = s.split(":");
    return parseInt(parts[0]) * 60 * 60 + parseInt(parts[1]) * 60 + parseInt(parts[2]);
}

export function sToTime(s) {
    s = Math.floor(s);
    const secs = s % 60;
    s = (s - secs) / 60;
    const mins = s % 60;
    const hrs = (s - mins) / 60;
    return ("0" + hrs).slice(-2) + ":" + ("0" + mins).slice(-2) + ":" + ("0" + secs).slice(-2);
}

export function sp(event) {
    event.stopPropagation();
}

export function makeTT(i, slider) {
    const tooltip = document.createElement("div");
    const input = document.createElement("input");

    input.className = "w-auto bg-secondary text-gray-400 p-0";
    // Add the input to the tooltip
    tooltip.className = "noUi-tooltip";
    tooltip.appendChild(input);

    // On change, set the slider
    input.addEventListener("change", function () {
        const values = [null, null];
        values[i] = timeToS(this.value);
        slider.noUiSlider.set(values);
    });

    // Catch all selections and make sure they don't reach the handle
    input.addEventListener("mousedown", sp);
    input.addEventListener("touchstart", sp);
    input.addEventListener("pointerdown", sp);
    input.addEventListener("MSPointerDown", sp);

    // Find the lower/upper slider handle and insert the tooltip
    slider.querySelector(i ? ".noUi-handle-upper" : ".noUi-handle-lower").appendChild(tooltip);
    return input;
}

export function toggleNewUnitForm(elem) {
    elem.classList.add("hidden");
    document.getElementById("unitNew").classList.remove("hidden");
}
