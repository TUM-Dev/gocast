// @ts-nocheck

const Button = videojs.getComponent('Button');

let skipTo = 0;
let progressRatio = 0;

/**
 * Button to add a class to passed in element that will toggle "theater mode" as defined
 * in app's CSS (larger player, dimmed background, etc...)
 */

class SkipSilenceToggle extends Button {
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

class TheaterModeToggle extends Button {

    constructor(player, options) {
        super(player, options);
        this.controlText('Big picture mode');
        this.el().firstChild.classList.add("vjs-icon-theater-toggle");
    }

    buildCSSClass() {
        if (document.getElementById(this.options_.elementToToggle).classList.contains(this.options_.className)) {
            return `vjs-theater-mode-control ${super.buildCSSClass()}`;
        } else {
            return `vjs-theater-mode-control ${super.buildCSSClass()}`;
        }
    }

    handleClick() {
        let theaterModeIsOn = document.getElementById(this.options_.elementToToggle).classList.toggle(this.options_.className);
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
const theaterMode = function (options) {
    this.ready(() => {
        // @ts-ignore
        this.addClass('vjs-theater-mode');
        let toggle = this.controlBar.addChild("theaterModeToggle", options);
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

const skipSilence = function (options) {
    this.ready(() => {
        // @ts-ignore
        this.addClass('vjs-skip-silence');
        let toggle = this.addChild("SkipSilenceToggle");
        toggle.el().classList.add("invisible")
        this.el().insertBefore(toggle.el(), this.bigPlayButton.el());

        let isShowing = false;
        const silences = JSON.parse(options)
        const len = silences.length
        let i = 0;
        let n = 0;
        this.on('timeupdate', () => {
            if (n++ % 5 != 0) {
                return;
            }
            n = 0;
            const ctime = this.currentTime();
            let shouldShow = false;
            for (i = 0; i < len; i++) {
                if (ctime >= silences[i].start && ctime < silences[i].end) {
                    shouldShow = true;
                    skipTo = silences[i].end
                    break
                }
            }
            if (isShowing && !shouldShow) {
                isShowing = false;
                toggle.el().classList.add("invisible");
            } else if (!isShowing && shouldShow) {
                isShowing = true;
                toggle.el().classList.remove("invisible");
            }
        });
    });
};


/**
 * @function watchProgress
 * Saves and retrieves the watch progress of the user as a fraction of the total watch time
 * @param streamID The ID of the currently watched stream
 */
const watchProgress = function (streamID: number) {
    this.ready(() => {
        postData("/api/progressRequest", {
            "streamID": streamID,
        }).then((data) => {
            if (data.status !== 200) {
                console.log(data);
            } else {
                data.text().then(data => {
                    const json = JSON.parse(data);
                    this.progressRatio = json["progress"];
                })
            }
        });

        let initialized = false;

        // Fetch the user's stream progress from the database and set the time on load
        this.on('loadedmetadata', () => {
            if (initialized) {
                return;
            }
            this.currentTime(this.progressRatio * this.duration());
            initialized = true;
        });

        // iPhone/iPad need to play the video first, so they depend on a different event
        // More info: https://www.w3.org/TR/html5/embedded-content-0.html#mediaevents
        this.on("canplaythrough", () => {
            if (initialized) {
                return;
            }
            this.currentTime(this.progressRatio * this.duration());
            initialized = true;
        });

        let lastChecked = new Date();

        // Fetch the user's stream progress from the database and set the time on load
        this.on('timeupdate', () => {
            const now = new Date();

            const diff = now - lastChecked;

            // Proceed with progress report every 10 seconds
            if (diff.valueOf() / 1000 < 10) {
                return;
            }

            const ctime = this.currentTime();
            const duration = this.duration();
            const progress = ctime / duration;

            postData("/api/progressReport", {
                "streamID": streamID,
                "progress": progress
            }).then(r => {
                    if (r.status !== 200) {
                        console.log(r);
                    }
                }
            );

            lastChecked = now;
        });
    });
};

// Register the plugin with video.js.
videojs.registerPlugin('theaterMode', theaterMode);
videojs.registerPlugin('skipSilence', skipSilence);
videojs.registerPlugin('watchProgress', watchProgress);
