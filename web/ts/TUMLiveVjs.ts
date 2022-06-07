import { postData, Section } from "./global";
import { StatusCodes } from "http-status-codes";
import videojs from "video.js";
import airplay from "@silvermine/videojs-airplay";
import dom = videojs.dom;

import { handleHotkeys } from "./hotkeys";

require("videojs-seek-buttons");
require("videojs-hls-quality-selector");
require("videojs-contrib-quality-levels");

const Button = videojs.getComponent("Button");
let player;

export function getPlayer() {
    return player;
}

/**
 * Initialize the player and bind it to a DOM object my-video
 */
export const initPlayer = function (
    autoplay: boolean,
    fluid: boolean,
    isEmbedded: boolean,
    playbackSpeeds: number[],
    courseName?: string,
    streamName?: string,
    streamUrl?: string,
    courseUrl?: string,
    streamStartIn?: number, // in seconds
) {
    player = videojs("my-video", {
        liveui: true,
        fluid: fluid,
        playbackRates: playbackSpeeds,
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
            hotkeys: handleHotkeys(),
        },
        autoplay: autoplay,
    });
    player.hlsQualitySelector();
    player.seekButtons({
        // TODO user preferences, e.g. change to 5s
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
        player.airPlay({
            addButtonToControlBar: true,
            buttonPositionIndex: -2,
        });

        const persistedVolume = window.localStorage.getItem("volume");
        if (persistedVolume !== null) {
            player.volume(persistedVolume);
        }
        const persistedMute = window.localStorage.getItem("muted");
        if (persistedMute !== null) {
            player.muted("true" === persistedMute);
        }
        if (isEmbedded) {
            player.addChild("Titlebar", {
                course: courseName,
                stream: streamName,
                courseUrl: courseUrl,
                streamUrl: streamUrl,
            });
        }
        if (streamStartIn > 0) {
            player.addChild("StartInOverlay", {
                course: courseName,
                stream: streamName,
                courseUrl: courseUrl,
                streamUrl: streamUrl,
                startIn: streamStartIn,
            });
        }
        player.addChild("OverlayIcon", {});
    });
    // handle hotkeys from anywhere on the page
    document.addEventListener("keydown", (event) => player.handleKeyDown(event));
};

let skipTo = 0;

/**
 * Button to add a class to passed player that will toggle skip silence button.
 */
export const SkipSilenceToggle = videojs.extend(Button, {
    constructor: function (...args) {
        Button.apply(this, args);
        this.controlText("Skip pause");
        (this.el().firstChild as HTMLElement).classList.add("icon-forward");
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

const Component = videojs.getComponent("Component");

export class Titlebar extends Component {
    // The constructor of a component receives two arguments: the
    // player it will be associated with and an object of options.
    constructor(player, options) {
        super(player, options);

        // If a `text` option was passed in, update the text content of
        // the component.
        this.updateTextContent(options);
    }

    // The `createEl` function of a component creates its DOM element.
    createEl() {
        return super.createEl("div", {
            // Prefixing classes of elements within a player with "vjs-"
            // is a convention used in Video.js.
            className: "vjs-title-bar",
        });
    }

    // This function could be called at any time to update the text
    // contents of the component.
    updateTextContent(options) {
        // Use Video.js utility DOM methods to manipulate the content
        // of the component's element.
        dom.emptyEl(this.el());

        this.el().innerHTML = `
        <div class="bg-gradient-to-b from-black/75 to-transparent pb-10 px-2 pt-2">
            <div class="flex">
            <div class="flex-grow">
                <h1>
                    <a target="_blank" class="text-gray-100 hover:text-white hover:underline" href="${
                        window.location.origin + options.streamUrl
                    }">${options.stream}</a>
                </h1>
                <h2 class="font-semibold">
                    <a target="_blank" class="text-gray-100 hover:text-white hover:underline" href="${
                        window.location.origin + options.courseUrl
                    }">${options.course}</a>
                </h2>
            </div>
            <div>
                <a target="_blank" href="${
                    window.location.origin + options.streamUrl
                }" class="inline-block text-gray-200 hover:text-white hover:underline">
                TUM-Live <i class="fas fa-external-link-alt"></i>
                </a>
            </div>
            </div>
        </div>
        `;
    }
}

export class StartInOverlay extends Component {
    // The constructor of a component receives two arguments: the
    // player it will be associated with and an object of options.
    constructor(player, options) {
        super(player, options);

        this.updateTextContent(options);
    }

    createEl() {
        return super.createEl("div", {
            className: "vjs-start-in-overlay",
        });
    }

    updateTextContent(options) {
        dom.emptyEl(this.el());
        if (options.startIn <= 0) {
            return;
        }

        this.el().innerHTML = `
        <div class="p-4 rounded bg-gray-900/75">
            <p><a target="_blank" href="${
                options.streamUrl
            }" class="text-gray-300 hover:text-white font-semibold text-m hover:underline">${options.stream}</a></p>
            <p><a target="_blank" href="${
                options.courseUrl
            }" class="text-gray-300 hover:text-white text-sm hover:underline">${options.course}</a></p>
            <p class="text-sm">Start in about <span class="font-semibold">${Math.floor(
                options.startIn / 60,
            )}</span> Minutes</p>
        </div>
        `;
        setTimeout(() => {
            options.startIn -= 10;
            this.updateTextContent(options);
        }, 10000);
    }
}

export class OverlayIcon extends Component {
    private removeIconTimeout;
    private readonly removeIconAfter;
    private wrapper;

    constructor(player, options) {
        super(player, options);
        this.removeIconAfter = options.removeIconAfter ?? 3000;
        this.setupEl_();
    }

    setupEl_() {
        this.wrapper = dom.createEl("div", { className: "vjs-overlay-icon-wrapper" });
        dom.appendContent(this.el(), this.wrapper);
    }

    createEl() {
        return super.createEl("div", {
            className: "vjs-overlay-icon-container",
        });
    }

    showIcon(className) {
        this.removeIcon();
        this.setupEl_();

        this.el().classList.add("vjs-overlay-icon-animate");
        this.removeIconTimeout = setTimeout(() => this.removeIcon(), this.removeIconAfter);

        dom.appendContent(this.wrapper, dom.createEl("i", { className }));
    }

    removeIcon() {
        clearTimeout(this.removeIconTimeout);
        dom.emptyEl(this.el());
        this.el().classList.remove("vjs-overlay-icon-animate");
    }
}

export function jumpTo(hours: number, minutes: number, seconds: number) {
    videojs("my-video").ready(() => {
        player.currentTime(toSeconds(hours, minutes, seconds));
    });
}

export class VideoSections {
    readonly streamID: number;

    list: Section[];
    currentHighlightIndex: number;

    constructor(streamID) {
        this.streamID = streamID;
        this.list = [];
        this.currentHighlightIndex = -1;
    }

    isCurrent(i: number): boolean {
        return this.currentHighlightIndex !== -1 && i === this.currentHighlightIndex;
    }

    async fetch() {
        await fetch(`/api/stream/${this.streamID}/sections`)
            .then((res: Response) => {
                if (!res.ok) {
                    throw new Error("Could not fetch sections");
                }
                return res.json();
            })
            .then((sections) => {
                this.list = sections;
                attachCurrentTimeEvent(this);
            })
            .catch((err) => {
                console.log(err);
                this.list = [];
                this.currentHighlightIndex = 0;
            });
    }
}

function attachCurrentTimeEvent(videoSection: VideoSections) {
    player.ready(() => {
        let timer;
        (function checkTimestamp() {
            timer = setTimeout(() => {
                hightlight(player, videoSection);
                checkTimestamp();
            }, 500);
        })();
        player.on("seeked", () => hightlight(player, videoSection));
    });
}

function hightlight(player, videoSection) {
    const currentTime = player.currentTime();
    videoSection.currentHighlightIndex = videoSection.list.findIndex((section, i, list) => {
        const next = list[i + 1];
        const sectionSeconds = toSeconds(section.startHours, section.startMinutes, section.startSeconds);
        return next === undefined || next === null // if last element and no next exists
            ? sectionSeconds <= currentTime
            : sectionSeconds <= currentTime &&
                  currentTime <= toSeconds(next.startHours, next.startMinutes, next.startSeconds) - 1;
    });
}

function toSeconds(hours: number, minutes: number, seconds: number): number {
    return hours * 60 * 60 + minutes * 60 + seconds;
}

// Register the plugin with video.js.
videojs.registerPlugin("skipSilence", skipSilence);
videojs.registerPlugin("watchProgress", watchProgress);
videojs.registerComponent("Titlebar", Titlebar);
videojs.registerComponent("StartInOverlay", StartInOverlay);
videojs.registerComponent("OverlayIcon", OverlayIcon);
airplay(videojs); //calls registerComponent internally
