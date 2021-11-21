import { postData } from './global'
import { StatusCodes } from "http-status-codes";

export module LectureUnits {
    let slider

    // @ts-ignore
    let player = videojs('my-video', {
    html5: {
        hls: {
            overrideNative: true
        },
        nativeVideoTracks: false,
        nativeAudioTracks: false,
        nativeTextTracks: false
    }
    });
    player.play()
    player.one("loadedmetadata", function () {
    slider = document.getElementById('sliderNew');
    // @ts-ignore
    noUiSlider.create(slider, {
        format: {
            to: function (value) {
                return sToTime(value)
            },
            from: function (value) {
                return value
            }
        },
        start: [0, player.duration()],
        connect: true,
        range: {
            'min': 0,
            'max': player.duration()
        }
    });

    let tooltipInputs = [makeTT(0, slider), makeTT(1, slider)];
    // @ts-ignore
    slider.noUiSlider.on('update', function (values, handle) {
        tooltipInputs[handle].value = values[handle];
        player.currentTime(timeToS(values[handle]));
    });
    })

    export function submitNewUnit(lectureID: number) {
    //convert from and to to milliseconds relatively to beginning of video.
    let from = timeToS(slider.noUiSlider.get()[0]) * 1000;
    let to = timeToS(slider.noUiSlider.get()[1]) * 1000;
    let title = (document.getElementById("newUnitTitle") as HTMLInputElement).value
    let description = (document.getElementById("newUnitDescription") as HTMLInputElement).value
    postData("/api/addUnit", {
        "lectureID": lectureID,
        "from": from,
        "to": to,
        "title": title,
        "description": description,
    }).then(data => {
        if (data.status == StatusCodes.OK) {
            window.location.reload()
        } else {
            data.text().then(
                text => {
                    alert("error! status: " + data.status + ", message: " + text)
                }
            )
        }
    })
    return false
    }

    export function submitCut(lectureID: number, courseID: number) {
    let from = timeToS(slider.noUiSlider.get()[0]) * 1000;
    let to = timeToS(slider.noUiSlider.get()[1]) * 1000;
    postData("/api/submitCut", {
        "lectureID": lectureID,
        "from": from,
        "to": to,
    }).then(data => {
        if (data.status == StatusCodes.OK) {
            window.location.replace("/admin/course/" + courseID)
        } else {
            data.text().then(
                text => {
                    alert("error! status: " + data.status + ", message: " + text)
                }
            )
        }
    })
    return false
    }

    export function deleteUnit(unitID: number) {
    postData("/api/deleteUnit/" + unitID).then(r => {
        if (r.status == StatusCodes.OK) {
            window.location.reload()
        }
    })
    }

    function timeToS(s) {
    let parts = s.split(":")
    return parseInt(parts[0]) * 60 * 60 + parseInt(parts[1]) * 60 + parseInt(parts[2])
    }

    function sToTime(s) {
    s = Math.floor(s)
    let secs = s % 60;
    s = (s - secs) / 60;
    let mins = s % 60;
    let hrs = (s - mins) / 60;
    return ("0" + hrs).slice(-2) + ':' + ("0" + mins).slice(-2) + ':' + ("0" + secs).slice(-2);
    }

    function sp(event) {
    event.stopPropagation();
    }

    function makeTT(i, slider) {
    let tooltip = document.createElement('div');
    let input = document.createElement('input');

    input.className = 'w-auto bg-secondary text-gray-400 p-0';
    // Add the input to the tooltip
    tooltip.className = 'noUi-tooltip';
    tooltip.appendChild(input);

    // On change, set the slider
    input.addEventListener('change', function () {
        const values = [null, null];
        values[i] = timeToS(this.value);
        slider.noUiSlider.set(values)
    });

    // Catch all selections and make sure they don't reach the handle
    input.addEventListener('mousedown', sp);
    input.addEventListener('touchstart', sp);
    input.addEventListener('pointerdown', sp);
    input.addEventListener('MSPointerDown', sp);

    // Find the lower/upper slider handle and insert the tooltip
    slider.querySelector(i ? '.noUi-handle-upper' : '.noUi-handle-lower').appendChild(tooltip);
    return input;
    }

    function toggleNewUnitForm(elem) {
    elem.classList.add("hidden")
    document.getElementById("unitNew").classList.remove("hidden")
    }
}
