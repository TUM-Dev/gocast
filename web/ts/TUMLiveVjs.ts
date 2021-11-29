// @ts-nocheck
import { StatusCodes } from "http-status-codes";
import { postData } from "./global";
import videojs from 'video.js';

const Button = videojs.getComponent('Button');

let skipTo = 0;

/**
 * Button to add a class to passed in element that will toggle "theater mode" as defined
 * in app's CSS (larger player, dimmed background, etc...)
 */

export class SkipSilenceToggle extends Button {
    private p;

    constructor(player, options) {
        this.p = player;
        super(player, options);
        this.controlText('Skip pause');
        this.el().firstChild.classList.add("icon-forward")
    }

    buildCSSClass() {
        return `vjs-skip-silence-control ${super.buildCSSClass()}`;
    }

    handleClick() {
        this.p.currentTime(skipTo);
    }
}

export class TheaterModeToggle extends Button {

    constructor(player, options) {
        super(player, options);
        this.controlText('Big picture mode');
        this.el().firstChild.classList.add("vjs-icon-theater-toggle");
    }

    buildCSSClass() {
        return `vjs-theater-mode-control ${super.buildCSSClass()}`;
    }

    handleClick() {
        const theaterModeIsOn = document.getElementById(this.options_.elementToToggle).classList.toggle(this.options_.className);
        this.player().trigger('theaterMode', {'theaterModeIsOn': theaterModeIsOn});

        if (theaterModeIsOn) {
            document.getElementById("watchContent").classList.remove("md:w-4/6", "lg:w-8/12", "2xl:max-w-screen-xl")
            this.player().fluid(false);
        } else {
            document.getElementById("watchContent").classList.add("md:w-4/6", "lg:w-8/12", "2xl:max-w-screen-xl")
            this.player().fluid(true);
        }
    }
}

videojs.registerComponent('TheaterModeToggle', TheaterModeToggle);
videojs.registerComponent('SkipSilenceToggle', SkipSilenceToggle);


/**
 * @function theaterMode
 * @param    {Object} [options={}]
 *           elementToToggle, the name of the DOM element to add/remove the 'theater-mode' CSS class
 */
export const theaterMode = function (options) {
    this.ready(() => {
        this.addClass('vjs-theater-mode');
        const toggle = this.controlBar.addChild("theaterModeToggle", options);
        this.controlBar.el().insertBefore(toggle.el(), this.controlBar.fullscreenToggle.el());
    });

    this.on('fullscreenchange', () => {
        if (this.isFullscreen()) {
            this.controlBar.getChild("theaterModeToggle").hide();
        } else {
            this.controlBar.getChild("theaterModeToggle").show();
        }
    });
};

export const skipSilence = function (options) {
    this.ready(() => {
        this.addClass('vjs-skip-silence');
        const toggle = this.addChild("SkipSilenceToggle");
        toggle.el().classList.add("invisible");
        this.el().insertBefore(toggle.el(), this.bigPlayButton.el());

        let isShowing = false;
        const silences = JSON.parse(options);
        const len = silences.length;
        const intervalMillis = 100;

        let i = 0;
        let timer;

        // Triggered when user presses play
        this.on('play', () => {
            timer = setInterval(() => { toggleSkipSilence() }, intervalMillis);
        });

        const toggleSkipSilence = () => {
            const ctime = this.currentTime();
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
        }

        // Triggered on pause and skipping the video
        this.on('pause', () => {
            clearInterval(timer);
        });

        // Triggered when the video has no time left
        this.on('ended', () => {
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
export const watchProgress = function (streamID: number, lastProgress: number, player) {
    player.ready(() => {
        let duration;
        let timer;
        let iOSReady = false;
        let intervalMillis = 10000;

        // Fetch the user's video progress from the database and set the time in the player
        this.on('loadedmetadata', () => {
            duration = this.duration();
            player.currentTime(lastProgress * duration);
        });

        // iPhone/iPad need to set the progress again when they actually play the video. That's why loadedmetadata is
        // not sufficient here.
        // See https://stackoverflow.com/questions/28823567/how-to-set-currenttime-in-video-js-in-safari-for-ios.
        if (videojs.browser.IS_IOS) {
            player.on('canplaythrough', () => {
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
                "streamID": streamID,
                "progress": progress
            }).then(r => {
                    if (r.status !== 200) {
                        console.log(r);
                        intervalMillis *= 2; // Binary exponential backoff for load balancing
                    }
                }
            );
        }

        // Triggered when user presses play
        this.on('play', () => {
            timer = setInterval(() => { reportProgress() }, intervalMillis);
        });

        // Triggered on pause and skipping the video
        this.on('pause', () => {
            clearInterval(timer);
            // "Bug" on iOS: The video is automatically paused at the beginning
            if (!iOSReady && videojs.browser.IS_IOS) {
                return;
            }
            reportProgress();
        });

        // Triggered when the video has no time left
        this.on('ended', () => {
            clearInterval(timer);
        });
    });
};

// Register the plugin with video.js.
videojs.registerPlugin('theaterMode', theaterMode);
videojs.registerPlugin('skipSilence', skipSilence);
videojs.registerPlugin('watchProgress', watchProgress);
