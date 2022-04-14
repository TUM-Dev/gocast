import { postData } from "./global";
import { StatusCodes } from "http-status-codes";
import videojs from "video.js";

require("videojs-seek-buttons");
require("videojs-hls-quality-selector");
require("videojs-contrib-quality-levels");

const Button = videojs.getComponent("Button");
let player;

/**
 * Initialize the player and bind it to a DOM object my-video
 */
export const initPlayer = function () {
    player = videojs("my-video", {
        liveui: true,
        fluid: false,
        playbackRates: [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2],
        html5: {
            reloadSourceOnError: true,
            vhs: {
                overrideNative: !videojs.browser.IS_SAFARI,
            },
            nativeVideoTracks: false,
            nativeAudioTracks: false,
            nativeTextTracks: false,
        },
        userActions: {
            hotkeys: {},
        },
        //nativeControlsForTouch: true,a
    });
    player.hlsQualitySelector();
    player.play();
    player.seekButtons({
        backIndex: 0,
        forward: 15,
        back: 15,
    });
    // handle volume store:
    player.on("volumechange", function () {
        window.localStorage.setItem("volume", player.volume());
        window.localStorage.setItem("muted", player.muted());
    });
    player.ready(function () {
        const persistedVolume = window.localStorage.getItem("volume");
        if (persistedVolume !== null) {
            player.volume(persistedVolume);
        }
        const persistedMute = window.localStorage.getItem("muted");
        if (persistedMute !== null) {
            player.muted("true" === persistedMute);
        }
    });
};

let skipTo = 0;

/**
 * Button to add a class to passed player that will toggle skip silence button.
 */
export const SkipSilenceToggle = videojs.extend(Button, {
    constructor: function (...args) {
        Button.apply(this, args);
        this.controlText("Skip pause");
        this.el().firstChild.classList.add("icon-forward");
    },
    handleClick: function () {
        videojs("my-video").currentTime(skipTo);
    },
    buildCSSClass: function () {
        return `vjs-skip-silence-control`;
    },
});

videojs.registerComponent("SkipSilenceToggle", SkipSilenceToggle);

export const skipSilence = function (options) {
    player.ready(() => {
        player.addClass("vjs-skip-silence");
        const toggle = player.addChild("SkipSilenceToggle");
        toggle.el().classList.add("invisible");
        player.el().insertBefore(toggle.el(), player.bigPlayButton.el());

        let isShowing = false;
        const silences = JSON.parse(options);
        const len = silences.length;
        const intervalMillis = 100;

        let i = 0;
        let timer;

        // Triggered when user presses play
        player.on("play", () => {
            timer = setInterval(() => {
                toggleSkipSilence();
            }, intervalMillis);
        });

        const toggleSkipSilence = () => {
            const ctime = player.currentTime();
            let shouldShow = false;
            for (i = 0; i < len; i++) {
                if (ctime >= silences[i].start && ctime < silences[i].end) {
                    shouldShow = true;
                    skipTo = silences[i].end;
                    break;
                }
            }
            if (isShowing && !shouldShow) {
                console.log("Not showing.");
                isShowing = false;
                toggle.el().classList.add("invisible");
            } else if (!isShowing && shouldShow) {
                console.log("Showing.");
                isShowing = true;
                toggle.el().classList.remove("invisible");
            }
        };

        // Triggered on pause and skipping the video
        player.on("pause", () => {
            clearInterval(timer);
        });

        // Triggered when the video has no time left
        player.on("ended", () => {
            clearInterval(timer);
        });
    });
};

/**
 * @function watchProgress
 * Saves and retrieves the watch progress of the user as a fraction of the total watch time
 * @param streamID The ID of the currently watched stream
 * @param lastProgress The last progress fetched from the database
 */
export const watchProgress = function (streamID: number, lastProgress: number) {
    const player = videojs("my-video");
    player.ready(() => {
        let duration;
        let timer;
        let iOSReady = false;
        let intervalMillis = 10000;

        // Fetch the user's video progress from the database and set the time in the player
        player.on("loadedmetadata", () => {
            duration = player.duration();
            player.currentTime(lastProgress * duration);
        });

        // iPhone/iPad need to set the progress again when they actually play the video. That's why loadedmetadata is
        // not sufficient here.
        // See https://stackoverflow.com/questions/28823567/how-to-set-currenttime-in-video-js-in-safari-for-ios.
        if (videojs.browser.IS_IOS) {
            player.on("canplaythrough", () => {
                // Can be executed multiple times during playback
                if (!iOSReady) {
                    player.currentTime(lastProgress * duration);
                    iOSReady = true;
                }
            });
        }

        const reportProgress = () => {
            const progress = player.currentTime() / duration;
            postData("/api/progressReport", {
                streamID: streamID,
                progress: progress,
            }).then((r) => {
                if (r.status !== StatusCodes.OK) {
                    console.log(r);
                    intervalMillis *= 2; // Binary exponential backoff for load balancing
                }
            });
        };

        // Triggered when user presses play
        player.on("play", () => {
            // See https://developer.mozilla.org/en-US/docs/Web/API/setInterval#ensure_that_execution_duration_is_shorter_than_interval_frequency
            (function reportNextProgress() {
                timer = setTimeout(function () {
                    reportProgress();
                    reportNextProgress();
                }, intervalMillis);
            })();
        });

        // Triggered on pause and skipping the video
        player.on("pause", () => {
            clearInterval(timer);
            // "Bug" on iOS: The video is automatically paused at the beginning
            if (!iOSReady && videojs.browser.IS_IOS) {
                return;
            }
            reportProgress();
        });

        // Triggered when the video has no time left
        player.on("ended", () => {
            clearInterval(timer);
        });
    });
};

export function jumpTo(hours: number, minutes: number, seconds: number) {
    videojs("my-video").ready(() => {
        player.currentTime(hours * 60 * 60 + minutes * 60 + seconds);
    });
}

// Register the plugin with video.js.
videojs.registerPlugin("skipSilence", skipSilence);
videojs.registerPlugin("watchProgress", watchProgress);
