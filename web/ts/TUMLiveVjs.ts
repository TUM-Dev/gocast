import { getQueryParam, keepQuery, postData, Time } from "./global";
import { VideoSectionList } from "./video-sections";
import { StatusCodes } from "http-status-codes";
import videojs from "video.js";
import airplay from "@silvermine/videojs-airplay";
import { loadAndSetTrackbars } from "./track-bars";

import { handleHotkeys } from "./hotkeys";
import dom = videojs.dom;

require("videojs-sprite-thumbnails");
require("videojs-seek-buttons");
require("videojs-contrib-quality-levels");

const Button = videojs.getComponent("Button");

const players = [];

export function getPlayers() {
    return players;
}

/**
 * Initialize the player and bind it to a DOM object my-video
 */
export const initPlayer = function (
    id: string,
    autoplay: boolean,
    fluid: boolean,
    isEmbedded: boolean,
    playbackSpeeds: number[],
    live: boolean,
    spriteID?: number,
    spriteInterval?: number,
    streamID?: number,
    courseName?: string,
    streamName?: string,
    streamUrl?: string,
    courseUrl?: string,
    streamStartIn?: number, // in seconds
) {
    const player = videojs(id, {
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
    }) as any;
    const isMobile = window.matchMedia && window.matchMedia("only screen and (max-width: 480px)").matches;
    if (spriteID && !isMobile) {
        player.spriteThumbnails({
            interval: spriteInterval,
            url: `/api/stream/${streamID}/thumbs/${spriteID}`,
            width: 160,
            height: 90,
        });
    }
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
    // handle rate store:
    player.on("ratechange", function () {
        window.localStorage.setItem("rate", player.playbackRate());
    });

    // When catching up to live, resume at normal speed
    player.liveTracker.on("liveedgechange", function (evt) {
        if (player.liveTracker.atLiveEdge() && player.playbackRate() > 1) {
            player.playbackRate(1);
        }
    });
    player.ready(function () {
        loadAndSetTrackbars(player, streamID);
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
        if (!live) {
            const persistedRate = window.localStorage.getItem("rate");
            if (persistedRate !== null) {
                player.playbackRate(persistedRate);
            }
        } else {
            // same procedure as with watch progress
            let iOSReady;
            const jumpTo: number | undefined = +getQueryParam("t");
            player.on("loadedmetadata", () => {
                if (jumpTo) {
                    player.currentTime(jumpTo);
                }
            });
            if (videojs.browser.IS_IOS) {
                player.on("canplaythrough", () => {
                    // Can be executed multiple times during playback
                    if (!iOSReady && jumpTo) {
                        player.currentTime(jumpTo);
                        iOSReady = true;
                    }
                });
            }
        }
        if (isEmbedded) {
            player.addChild("Titlebar", {
                course: courseName,
                stream: streamName,
                courseUrl: courseUrl,
                streamUrl: streamUrl,
            });
        }

        if (spriteID) {
            const timeTooltip = player
                .getChild("controlBar")
                .getChild("progressControl")
                .getChild("seekBar")
                .getChild("mouseTimeDisplay")
                .getChild("timeTooltip");
            if (timeTooltip) {
                timeTooltip.el().classList.add("thumb");
            }
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
    players.push(player);
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
        for (let i = 0; i < players.length; i++) {
            players[i].currentTime(skipTo);
        }
    },
    buildCSSClass: function () {
        return `vjs-skip-silence-control`;
    },
});

videojs.registerComponent("SkipSilenceToggle", SkipSilenceToggle);

export const skipSilence = function (options) {
    for (let j = 0; j < players.length; j++) {
        players[j].ready(() => {
            players[j].addClass("vjs-skip-silence");
            const toggle = players[j].addChild("SkipSilenceToggle");
            toggle.el().classList.add("invisible");
            players[j].el().insertBefore(toggle.el(), players[j].bigPlayButton.el());

            let isShowing = false;
            const silences = JSON.parse(options);
            const len = silences.length;
            const intervalMillis = 100;

            let i = 0;
            let timer;

            // Triggered when user presses play
            players[j].on("play", () => {
                timer = setInterval(() => {
                    toggleSkipSilence();
                }, intervalMillis);
            });

            const toggleSkipSilence = () => {
                const ctime = players[j].currentTime();
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
            players[j].on("pause", () => {
                clearInterval(timer);
            });

            // Triggered when the video has no time left
            players[j].on("ended", () => {
                clearInterval(timer);
            });
        });
    }
};

/**
 * @function watchProgress
 * Saves and retrieves the watch progress of the user as a fraction of the total watch time
 * If query parameter 't' is specified, the timestamp given by 't' will be used.
 * @param streamID The ID of the currently watched stream
 * @param lastProgress The last progress fetched from the database
 */
export const watchProgress = function (streamID: number, lastProgress: number) {
    const j = 0;
    players[j].ready(() => {
        let duration;
        let timer;
        let iOSReady = false;
        let intervalMillis = 10000;
        let jumpTo: number;

        // Fetch the user's video progress from the database and set the time in the player
        players[j].on("loadedmetadata", () => {
            duration = players[j].duration();
            jumpTo = +getQueryParam("t") || lastProgress * duration;
            players[j].currentTime(jumpTo);
        });

        // iPhone/iPad need to set the progress again when they actually play the video. That's why loadedmetadata is
        // not sufficient here.
        // See https://stackoverflow.com/questions/28823567/how-to-set-currenttime-in-video-js-in-safari-for-ios.
        if (videojs.browser.IS_IOS) {
            players[j].on("canplaythrough", () => {
                // Can be executed multiple times during playback
                if (!iOSReady) {
                    players[j].currentTime(jumpTo);
                    iOSReady = true;
                }
            });
        }

        const reportProgress = () => {
            const progress = players[j].currentTime() / duration;
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

        //Triggered when user presses play
        players[j].on("play", () => {
            // See https://developer.mozilla.org/en-US/docs/Web/API/setInterval#ensure_that_execution_duration_is_shorter_than_interval_frequency
            (function reportNextProgress() {
                timer = setTimeout(function () {
                    reportProgress();
                    reportNextProgress();
                }, intervalMillis);
            })();
        });

        // Triggered on pause and skipping the video
        players[j].on("pause", () => {
            clearInterval(timer);
            // "Bug" on iOS: The video is automatically paused at the beginning
            if (!iOSReady && videojs.browser.IS_IOS) {
                return;
            }
            reportProgress();
        });

        // Triggered when the video has no time left
        players[j].on("ended", () => {
            clearInterval(timer);
        });
    });
};

/**
 * @function syncTime
 * Sets the currentTime of all players to the currentTime of the j-th player without triggering any events.
 * @param j Player index
 */
function syncTime(j: number) {
    const t = players[j].currentTime();

    // Seek all players to timestamp t
    for (let k = 0; k < players.length; k++) {
        if (k == j) continue;
        players[k].currentTime(t);
    }
}

let lastSeek = 0;

function throttle(func, timeFrame) {
    return () => {
        const now = Date.now();
        if (now - lastSeek >= timeFrame) {
            func();
            lastSeek = now;
        }
    };
}

/**
 * Adds the necessary event listeners for syncing the players.
 * @param j Player index
 */
const addEventListenersForSyncing = function (j: number) {
    players[j].on("play", () => {
        for (let k = 0; k < players.length; k++) {
            if (k == j) continue;

            const playPromise = players[k].play();
            if (playPromise !== undefined) {
                playPromise
                    .then((_) => {
                        // Playback started
                    })
                    .catch((_) => {
                        for (let k = 0; k < players.length; k++) {
                            players[k].pause();
                        }
                    });
            }
        }
    });

    players[j].on("ratechange", () => {
        for (let k = 0; k < players.length; k++) {
            if (k == j) continue;
            players[k].playbackRate(players[j].playbackRate());
        }
    });

    players[j].on("pause", () => {
        for (let k = 0; k < players.length; k++) {
            if (k == j) continue;
            players[k].pause();
        }
    });

    players[j].on(
        "seeked",
        throttle(() => syncTime(j), 2000),
    );

    players[j].on("waiting", () => {
        for (let k = 0; k < players.length; k++) {
            if (k == j) continue;
            players[k].pause();
        }
        players[j].one("canplay", () => players[j].play());
    });

    players[j].on("ended", () => {
        for (let k = 0; k < players.length; k++) {
            if (k == j) continue;
            players[k].pause();
        }
    });
};

/**
 * @function syncPlayers
 * Adds event listeners to all players for syncing.
 */
export const syncPlayers = function () {
    players[0].ready(() => {
        for (let j = 0; j < players.length; j++) {
            addEventListenersForSyncing(j);
        }
    });
};

/**
 * Registers a time watcher that observes the time of the current player
 * @param callBack call back function responsible for handling player time updates
 * @return callBack function that got registered for listening to player time updates (used to deregister)
 */
export const registerTimeWatcher = function (callBack: (currentPlayerTime: number) => void): () => void {
    const timeWatcherCallBack: () => void = () => {
        callBack(players[0].currentTime());
    };
    players[0]?.on("timeupdate", timeWatcherCallBack);
    return timeWatcherCallBack;
};

/**
 * Deregisters a time watching obeserver from the current player
 * @param callBackToDeregister regestered callBack function
 */
export const deregisterTimeWatcher = function (callBackToDeregister: () => void) {
    players[0]?.off("timeupdate", callBackToDeregister);
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
    for (let j = 0; j < players.length; j++) {
        players[j].ready(() => {
            players[j].currentTime(new Time(hours, minutes, seconds).toSeconds());
        });
    }
}


export function jumpToMs(ms: number) {
    for (let j = 0; j < players.length; j++) {
        players[j].ready(() => {
            players[j].currentTime(ms/1000);
        });
    }
}

type SeekLoggerLogFunction = (position: number) => void;
const SEEK_LOGGER_DEBOUNCE_TIMEOUT = 4000;

export class SeekLogger {
    readonly streamID: number;
    log: SeekLoggerLogFunction;

    initialSeekDone = false;

    constructor(streamID) {
        this.streamID = parseInt(streamID);
        this.log = debounce(
            (position) => postData(`/api/seekReport/${this.streamID}`, { position }),
            SEEK_LOGGER_DEBOUNCE_TIMEOUT,
        );
    }

    attach() {
        players[0].ready(() => {
            players[0].on("seeked", () => {
                if (this.initialSeekDone) {
                    return this.log(players[0].currentTime());
                }
                this.initialSeekDone = true;
            });

            // If there is no initial seek, reset after 3 second
            setTimeout(() => (this.initialSeekDone = true), 3000);
        });
    }
}

export function attachCurrentTimeEvent(videoSection: VideoSectionList) {
    for (let j = 0; j < players.length; j++) {
        players[j].ready(() => {
            let timer;
            (function checkTimestamp() {
                timer = setTimeout(() => {
                    highlight(players[j], videoSection);
                    checkTimestamp();
                }, 500);
            })();
            players[j].on("seeked", () => highlight(players[j], videoSection));
        });
    }
}

export function switchView(baseUrl: string) {
    const isDVR = getQueryParam("dvr") === "";

    let redirectUrl = keepQuery(baseUrl);
    if (isDVR) {
        const player = getPlayers()[0];
        const url = new URL(window.location.origin + redirectUrl);
        url.searchParams.set("t", String(Math.floor(player.currentTime())));
        redirectUrl = url.toString();
    }
    window.location.assign(redirectUrl);
}

function highlight(player, videoSection) {
    const currentTime = player.currentTime();
    videoSection.currentHighlightIndex = videoSection.list.findIndex((section, i, list) => {
        const next = list[i + 1];
        const sectionSeconds = new Time(section.startHours, section.startMinutes, section.startSeconds).toSeconds();
        return next === undefined || next === null // if last element and no next exists
            ? sectionSeconds <= currentTime
            : sectionSeconds <= currentTime &&
                  currentTime <= new Time(next.startHours, next.startMinutes, next.startSeconds).toSeconds() - 1;
    });
}

function debounce(func, timeout) {
    let timer;
    return (...args) => {
        clearTimeout(timer);
        timer = setTimeout(() => func.apply(this, args), timeout);
    };
}

// Register the plugin with video.js.
videojs.registerPlugin("skipSilence", skipSilence);
videojs.registerPlugin("watchProgress", watchProgress);
videojs.registerComponent("Titlebar", Titlebar);
videojs.registerComponent("StartInOverlay", StartInOverlay);
videojs.registerComponent("OverlayIcon", OverlayIcon);
airplay(videojs); //calls registerComponent internally
