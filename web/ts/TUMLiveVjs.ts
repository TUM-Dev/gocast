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
        fluid: true,
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

    player.theaterMode({ elementToToggle: "my-video", className: "theater-mode" });
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
 * Button to add a class to passed in element that will toggle "theater mode" as defined
 * in app's CSS (larger player, dimmed background, etc...)
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

export const TheaterModeToggle = videojs.extend(Button, {
    constructor: function (...args) {
        Button.apply(this, args);
        this.controlText("Big picture mode");
        this.el().firstChild.classList.add("vjs-icon-theater-toggle");
    },
    handleClick: function () {
        const theaterModeIsOn = document.getElementById("my-video").classList.toggle("theater-mode");
        videojs("my-video").trigger("theaterMode", { theaterModeIsOn: theaterModeIsOn });

        if (theaterModeIsOn) {
            document.getElementById("watchContent").classList.remove("md:w-4/6", "lg:w-8/12", "2xl:max-w-screen-xl");
            videojs("my-video").fluid(false);
        } else {
            document.getElementById("watchContent").classList.add("md:w-4/6", "lg:w-8/12", "2xl:max-w-screen-xl");
            videojs("my-video").fluid(true);
        }
    },
    buildCSSClass: function () {
        return `vjs-theater-mode-control`;
    },
});

videojs.registerComponent("TheaterModeToggle", TheaterModeToggle);
videojs.registerComponent("SkipSilenceToggle", SkipSilenceToggle);

/**
 * @function theaterMode
 * @param    {Object} [options={}]
 *           elementToToggle, the name of the DOM element to add/remove the 'theater-mode' CSS class
 */
export const theaterMode = function (options) {
    const player = videojs("my-video");
    player.ready(() => {
        player.addClass("vjs-theater-mode");
        const toggle = player.controlBar.addChild("theaterModeToggle", options);
        player.controlBar.el().insertBefore(toggle.el(), player.controlBar.fullscreenToggle.el());
    });

    player.on("fullscreenchange", () => {
        if (player.isFullscreen()) {
            player.controlBar.getChild("theaterModeToggle").hide();
        } else {
            player.controlBar.getChild("theaterModeToggle").show();
        }
    });
};

export const skipSilence = function (options) {
    const player = videojs("my-video");
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

// Register the plugin with video.js.
videojs.registerPlugin("theaterMode", theaterMode);
videojs.registerPlugin("skipSilence", skipSilence);
videojs.registerPlugin("watchProgress", watchProgress);
