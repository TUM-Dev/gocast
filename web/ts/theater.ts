// @ts-nocheck
const Button = videojs.getComponent('Button');
const defaults = {className: 'theater-mode'};
const componentName = 'theaterModeToggle';

/**
 * Button to add a class to passed in element that will toggle "theater mode" as defined
 * in app's CSS (larger player, dimmed background, etc...)
 */

class TheaterModeToggle extends Button {

    constructor(player, options) {
        super(player, options);
        this.controlText('Big picture mode');
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
            document.getElementById("watchWrapper").classList.remove("md:w-4/6", "lg:w-8/12", "2xl:max-w-screen-xl")
            this.player().fluid(false);
        } else {
            document.getElementById("watchWrapper").classList.add("md:w-4/6", "lg:w-8/12", "2xl:max-w-screen-xl")
            this.player().fluid(true);
        }
    }
}

videojs.registerComponent('TheaterModeToggle', TheaterModeToggle);

const onPlayerReady = (player, options) => {
    player.addClass('vjs-theater-mode');

    let toggle = player.controlBar.addChild(componentName, options);
    player.controlBar.el().insertBefore(toggle.el(), player.controlBar.fullscreenToggle.el());
};

/**
 * @function theaterMode
 * @param    {Object} [options={}]
 *           elementToToggle, the name of the DOM element to add/remove the 'theater-mode' CSS class
 */
const theaterMode = function (options) {
    this.ready(() => {
        // @ts-ignore
        onPlayerReady(this, videojs.mergeOptions(defaults, options));
    });

    this.on('fullscreenchange', (event) => {
        if (this.isFullscreen()) {
            this.controlBar.getChild(componentName).hide();
        } else {
            this.controlBar.getChild(componentName).show();
        }
    });
};

// Register the plugin with video.js.
videojs.registerPlugin('theaterMode', theaterMode);
